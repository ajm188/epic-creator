package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"text/template"
)

import (
	"github.com/trivago/tgo/tcontainer"
	"gopkg.in/alecthomas/kingpin.v2"
	jira "gopkg.in/andygrunwald/go-jira.v1"
)

const (
	ticketsHelp = `Path to JSON file containing ticket parameters. It should conform to the following schema:

	[
		{
			"project": <my-project>,
			"params": {
				...
			}
		},
		...
	]

	where "params" will be passed as context to the
	issue-template (see below).
	For more information about golang templating, see
	the text/template package documentation at
	https://godoc.org/text/template.
`
)

func jiraAPIRequestErrorHandler(resp *jira.Response, err error) error {
	fmt.Fprintf(os.Stderr, "Request: %v\n", resp.Response.Request)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "Response body: %s\n", body)
	return err
}

type Ticket struct {
	Project string
	Params  map[string]interface{}
	CustomEpicField string `json:"custom_epic_field,omitempty"`
}

func loadTickets(ticketsFilePath string) ([]Ticket, error) {
	data, err := ioutil.ReadFile(ticketsFilePath)
	if err != nil {
		return nil, err
	}

	tickets := make([]Ticket, 0)
	err = json.Unmarshal(data, &tickets)
	return tickets, err
}

func loadTemplate(issueTemplate string) (*template.Template, error) {
	return template.ParseFiles(issueTemplate)
}

func createIssues(
	client *jira.Client,
	summaryTemplate *template.Template,
	descriptionTemplate *template.Template,
	tickets []Ticket,
	epic *jira.Epic,
) error {
	summaryBuf := bytes.NewBufferString("")
	descriptionBuf := bytes.NewBufferString("")

	projectCache := make(map[string]*jira.Project, 0)
	for _, ticket := range tickets {
		summaryBuf.Reset()
		descriptionBuf.Reset()

		_, ok := projectCache[ticket.Project]
		if !ok {
			project, resp, err := client.Project.Get(ticket.Project)
			if err != nil {
				return jiraAPIRequestErrorHandler(resp, err)
			}
			projectCache[ticket.Project] = project
		}
		project, _ := projectCache[ticket.Project]
		if len(project.IssueTypes) == 0 {
			fmt.Fprint(
				os.Stderr,
				"No issue types found for project %s - Skipping creating %v\n",
				ticket.Project,
				ticket,
			)
		}
		issueType := project.IssueTypes[0]

		ticket.Params["epic"] = epic.Key
		// write template into buf
		err := summaryTemplate.Execute(summaryBuf, ticket)
		if err != nil {
			return err
		}
		err = descriptionTemplate.Execute(descriptionBuf, ticket)
		if err != nil {
			return err
		}

		// create issue struct
		fields := jira.IssueFields{
			Summary:     summaryBuf.String(),
			Description: descriptionBuf.String(),
			Type: issueType,
			Project: *project,
		}
		if ticket.CustomEpicField != "" {
			fields.Unknowns = tcontainer.MarshalMap{
				ticket.CustomEpicField: epic.Key,
			}
		} else {
			fields.Epic = epic
		}
		issue := jira.Issue{Fields: &fields}

		// make request
		createdIssue, resp, err := client.Issue.Create(&issue)
		if err != nil {
			return jiraAPIRequestErrorHandler(resp, err)
		}
		fmt.Printf(
			"Created: %v\nIssue Fields: %v\n",
			*createdIssue,
			createdIssue.Fields,
		)
	}
	return nil
}

func getEpic(client *jira.Client, epicName string) (*jira.Epic, error) {
	issue, resp, err := client.Issue.Get(
		epicName,
		&jira.GetQueryOptions{
			Fields: "issuetype",
		},
	)

	if err != nil {
		return nil, jiraAPIRequestErrorHandler(resp, err)
	}

	if issue.Fields.Type.Name != "Epic" {
		return nil, err
	}

	id, err := strconv.Atoi(issue.ID)
	if err != nil {
		return nil, err
	}

	epic := &jira.Epic{
		ID: id,
		Self: issue.Self,
		Key: issue.Key,
	}
	return epic, nil
}

type Creds struct {
	User     string
	Password string
}

func getCreds(authFilePath string) (*Creds, error) {
	data, err := ioutil.ReadFile(authFilePath)
	if err != nil {
		return nil, err
	}

	var creds Creds
	err = json.Unmarshal(data, &creds)
	return &creds, err
}

func main() {
	workdir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	url := kingpin.Flag(
		"jira-url",
		"JIRA instance URL",
	).URL()
	authFilePath := kingpin.Flag(
		"auth-file",
		"Path to JSON file with auth credentials. Must have <user> and <password>.",
	).Default(
		path.Join(workdir, "auth.json"),
	).ExistingFile()
	ticketsFilePath := kingpin.Flag(
		"tickets-json",
		ticketsHelp,
	).Default(
		path.Join(workdir, "tickets.json"),
	).ExistingFile()
	summaryTemplatePath := kingpin.Flag(
		"summary-template",
		"Path to template to use for summary of Issues created in the Epic.",
	).Default(
		path.Join(workdir, "summary.jira.tmpl"),
	).ExistingFile()
	descriptionTemplatePath := kingpin.Flag(
		"description-template",
		"Path to template to use for description of Issues created in the Epic.",
	).Default(
		path.Join(workdir, "description.jira.tmpl"),
	).ExistingFile()
	epicName := kingpin.Arg("epic", "Epic to create issues in.").Required().String()

	kingpin.Parse()

	creds, err := getCreds(*authFilePath)
	if err != nil {
		panic(err)
	}

	client, err := jira.NewClient(nil, (*url).String())
	if err != nil {
		panic(err)
	}
	client.Authentication.SetBasicAuth(creds.User, creds.Password)

	summaryTemplate, err := loadTemplate(*summaryTemplatePath)
	if err != nil {
		panic(err)
	}
	descriptionTemplate, err := loadTemplate(*descriptionTemplatePath)
	if err != nil {
		panic(err)
	}

	tickets, err := loadTickets(*ticketsFilePath)
	if err != nil {
		panic(err)
	}
	epic, err := getEpic(client, *epicName)
	if err != nil {
		panic(err)
	}
	if epic == nil {
		fmt.Fprintf(os.Stderr, "Found %s but it was not an epic.\n", *epicName)
		panic(err)
	}

	err = createIssues(
		client,
		summaryTemplate,
		descriptionTemplate,
		tickets,
		epic,
	)
	if err != nil {
		panic(err)
	}
}
