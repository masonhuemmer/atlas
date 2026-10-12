package main

import (
	"context"
	"os"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/adapters/atlassian/rest"
	"github.com/masonhuemmer/atlas/internal/adapters/cli"
	"github.com/masonhuemmer/atlas/internal/adapters/keychain"
	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/config"
	"github.com/masonhuemmer/atlas/internal/domain"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(cli.Fail(cli.Deps{}, err))
	}
	d := cli.Deps{Config: cfg}
	if os.Getenv("ATLAS_FAKE") == "1" {
		mem := atlassian.Seed()
		d.Jira = atlassian.JiraAPI{Memory: mem}
		d.Confluence = atlassian.ConfluenceAPI{Memory: mem}
		d.PR = atlassian.PRAPI{Memory: mem}
		d.JSM = atlassian.JSMAPI{Memory: mem}
		d.Store = &keychain.FileStore{Path: keychain.FakeSessionPath()}
		d.Login = auth.FakeLogin()
	} else {
		switch os.Getenv("ATLAS_CREDENTIAL_STORE") {
		case "", "keyring":
			d.Store = &keychain.Fallback{
				Primary:   keychain.Keyring{},
				Secondary: &keychain.FileStore{Path: keychain.LiveSessionPath()},
			}
		case "file":
			d.Store = &keychain.FileStore{Path: keychain.LiveSessionPath()}
		default:
			os.Exit(cli.Fail(cli.Deps{}, domain.Usage("ATLAS_CREDENTIAL_STORE must be keyring or file")))
		}
		d.Login = storeLogin
		live := &rest.Client{Store: d.Store}
		d.Jira = rest.Jira{Client: live}
		d.Confluence = rest.Confluence{Client: live}
		d.PR = rest.Bitbucket{Client: live}
		d.JSM = rest.JSM{Client: live}
	}
	os.Exit(cli.Run(os.Args, d))
}

func storeLogin(_ context.Context, email, token string) (auth.Cred, error) {
	if email == "" || token == "" {
		return auth.Cred{}, domain.Usage("email and token are required").WithHint("atlas auth login --site ALIAS or --workspace WORKSPACE with --email EMAIL --token TOKEN")
	}
	return auth.Cred{Email: email, Token: token}, nil
}
