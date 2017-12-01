package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type spreadConfig struct {
	Token     string   `yaml:"token"`
	Owner     string   `yaml:"owner"`
	Repo      string   `yaml:"repo"`
	Self      string   `yaml:"self"`
	Assignees []string `yaml:"assignees"`
}

func confirm() bool {
	var s string
	fmt.Printf("(y/n): ")
	_, err := fmt.Scan(&s)
	if err != nil {
		panic(err)
	}
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	if s == "y" || s == "yes" {
		return true
	}
	return false
}

func main() {
	var config spreadConfig
	configFile, err := ioutil.ReadFile("spread-config.yaml")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	options := github.IssueListByRepoOptions{
		State:    "open",
		Assignee: config.Self,
	}
	issues, _, err := client.Issues.ListByRepo(ctx, config.Owner, config.Repo, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(issues), "assigned to", config.Self)

	assigneeIndex := 0
	for _, issue := range issues {
		assignee := config.Assignees[assigneeIndex]
		if issue.Title == nil {
			continue
		}
		fmt.Println(*issue.Title)
		fmt.Printf("reassign to %v?", assignee)
		if !confirm() {
			continue
		}

		_, _, err := client.Issues.AddAssignees(ctx, config.Owner, config.Repo, *issue.Number, []string{assignee})
		if err != nil {
			fmt.Println(err)
			continue
		}

		_, _, err = client.Issues.RemoveAssignees(ctx, config.Owner, config.Repo, *issue.Number, []string{config.Self})
		if err != nil {
			fmt.Println(err)
			continue
		}

		comment := fmt.Sprintf("@%v PTAL", assignee)
		_, _, err = client.Issues.CreateComment(ctx, config.Owner, config.Repo, *issue.Number, &github.IssueComment{Body: &comment})
		if err != nil {
			fmt.Println(err)
		}

		assigneeIndex++
		if assigneeIndex >= len(config.Assignees) {
			assigneeIndex = 0
		}
	}
}
