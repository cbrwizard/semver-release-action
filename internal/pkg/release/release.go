package release

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/cbrwizard/semver-release-action/internal/pkg/action"
	"github.com/google/go-github/v28/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const releaseTypeNone = "none"
const releaseTypeRelease = "release"
const releaseTypeTag = "tag"

type repository struct {
	owner string
	name  string
	token string
}

type releaseDetails struct {
	version string
	target  string
}

func Command() *cobra.Command {
	var releaseType string

	cmd := &cobra.Command{
		Use:  "release [REPOSITORY] [TARGET_COMMITISH] [VERSION] [GH_TOKEN] [GH_EVENT_PATH]",
		Args: cobra.ExactArgs(5),
		Run: func(cmd *cobra.Command, args []string) {
			execute(cmd, releaseType, args)
		},
	}

	cmd.Flags().StringVarP(&releaseType, "strategy", "s", releaseTypeRelease, "Release strategy")

	return cmd
}

func execute(cmd *cobra.Command, releaseType string, args []string) {
	parts := strings.Split(args[0], "/")
	repo := repository{
		owner: parts[0],
		name:  parts[1],
		token: args[3],
	}

	release := releaseDetails{
		version: args[2],
		target:  args[1],
	}
	pullRequestEvent := parseEvent(cmd, args[4])

	ctx := context.Background()

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: repo.token})
	client := github.NewClient(oauth2.NewClient(ctx, tokenSource))

	switch releaseType {
	case releaseTypeNone:
		return
	case releaseTypeRelease:
		if err := createGithubRelease(ctx, client, repo, release, *pullRequestEvent); err != nil {
			action.AssertNoError(cmd, err, "could not create GitHub release: %s", err)
		}
		return
	case releaseTypeTag:
		if err := createLightweightTag(ctx, client, repo, release); err != nil {
			action.AssertNoError(cmd, err, "could not create lightweight tag: %s", err)
		}
		return
	default:
		action.Fail(cmd, "unknown release strategy: %s", releaseType)
	}
}

func createLightweightTag(ctx context.Context, client *github.Client, repo repository, release releaseDetails) error {
	_, _, err := client.Git.CreateRef(ctx, repo.owner, repo.name, &github.Reference{
		Ref: github.String(fmt.Sprintf("refs/tags/%s", release.version)),
		Object: &github.GitObject{
			SHA: &release.target,
		},
	})

	return err
}

func createGithubRelease(ctx context.Context, client *github.Client, repo repository, release releaseDetails, pullRequest github.PullRequestEvent) error {
	log.Print("Pull request body:")
	log.Print(pullRequest.PullRequest.Body)
	_, _, err := client.Repositories.CreateRelease(ctx, repo.owner, repo.name, &github.RepositoryRelease{
		Name:            &release.version,
		TagName:         &release.version,
		TargetCommitish: &release.target,
		Body:            pullRequest.PullRequest.Body,
		Draft:           github.Bool(false),
		Prerelease:      github.Bool(false),
	})

	return err
}

func parseEvent(cmd *cobra.Command, filePath string) *github.PullRequestEvent {
	parsed, err := github.ParseWebHook("pull_request", readEvent(cmd, filePath))
	action.AssertNoError(cmd, err, "could not parse GitHub event: %s", err)

	event, ok := parsed.(*github.PullRequestEvent)
	if !ok {
		action.Fail(cmd, "could not parse GitHub event into a PullRequestEvent: %s", err)
	}

	return event
}

func readEvent(cmd *cobra.Command, filePath string) []byte {
	file, err := os.Open(filePath)
	action.AssertNoError(cmd, err, "could not open GitHub event file: %s", err)
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	action.AssertNoError(cmd, err, "could not read GitHub event file: %s", err)

	return b
}
