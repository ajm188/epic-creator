package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"
)

import (
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

type Ticket struct {
	Project string
	Params  map[string]interface{}
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
	for _, ticket := range tickets {
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
		issue := jira.Issue{
			Fields: &jira.IssueFields{
				Summary:     summaryBuf.String(),
				Description: descriptionBuf.String(),
				Project: jira.Project{
					Name: ticket.Project,
				},
				Epic: epic,
			},
		}

		/*
		// make request
		createdIssue, resp, err := client.Issue.Create(&issue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", resp.Response.Request)
			return err
		}
		fmt.Printf("Created: %v\n", createdIssue)
		*/
		fmt.Println(*issue.Fields)

		summaryBuf.Reset()
		descriptionBuf.Reset()
	}
	return nil
}

func ensureEpicExists(client *jira.Client, epic string) (bool, error) {
	issue, resp, err := client.Issue.Get(
		epic,
		&jira.GetQueryOptions{
			Fields: "issuetype",
		},
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", *resp.Response.Request)
		return false, err
	}

	return issue.Fields.Type.Name == "Epic", nil
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
	epic := kingpin.Arg("epic", "Epic to create issues in.").Required().String()

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
	haveEpic, err := ensureEpicExists(client, *epic)
	if err != nil || !haveEpic {
		fmt.Fprintf(
			os.Stderr,
			"Looking for epic %s with result %v.\n",
			*epic,
			haveEpic,
		)
		panic(err)
	}

	err = createIssues(
		client,
		summaryTemplate,
		descriptionTemplate,
		tickets,
		&jira.Epic{Key: *epic},
	)
	if err != nil {
		panic(err)
	}
}
