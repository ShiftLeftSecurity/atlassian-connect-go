package handling

import (
	"bytes"
	"log"
	"net/http"
	"testing"

	"github.com/ShiftLeftSecurity/atlassian-connect-go/storage"
	"github.com/beme/abide"
)

type tCase struct {
	name    string
	p       *Plugin
	wantErr bool
}

type fakeStore struct {
	j *storage.JiraInstallInformation
}

func (f *fakeStore) SaveJiraIntallInformation(j *storage.JiraInstallInformation) error {
	f.j = j
	return nil
}

func (f *fakeStore) JiraIntallInformation(clientKey string) (*storage.JiraInstallInformation, error) {
	return f.j, nil
}

func adaptLogger(t *testing.T) *log.Logger {
	return log.New(&tlog{t: t}, "TEST:", log.LstdFlags)
}

type tlog struct {
	t *testing.T
}

func (t *tlog) Write(p []byte) (int, error) {
	t.t.Logf(string(p))
	return len(p), nil
}

func TestPlugin_renderAtlassianConnectJSON(t *testing.T) {
	l := adaptLogger(t)
	tests := []tCase{
		func() tCase {
			p := NewPlugin("test_atlassian_connect_01",
				"a test of generating atlassian connect",
				"io.something.very.uniqye", "https://invalidurl.shiftleft.io",
				"/path/to/api",
				&fakeStore{}, l,
				[]string{"READ", "WRITE", "ACT_AS_USER"},
				Vendor{
					Name: "ShiftLeft",
					URL:  "https://www.shiftleft.io",
				})
			var fakeHandleFunc = func(jii *storage.JiraInstallInformation, s storage.Store, w http.ResponseWriter, r *http.Request) {}
			err := p.AddLifecycleEvent(LCInstalled, "/install", fakeHandleFunc)
			if err != nil {
				t.Error(err)
			}

			err = p.AddWebhook("jira:issue_updated", NewRoutePath("/issue_updated", map[string]string{}), fakeHandleFunc)
			if err != nil {
				t.Error(err)
			}

			err = p.AddJiraIssueField(JiraIssueFields{
				Description: Description{Value: "A more detailed description"},
				Key:         "A_Field",
				Name: Name{
					Value: "AFancyFunc",
				},
				Type: "text"}) // https://developer.atlassian.com/cloud/jira/platform/modules/issue-field/
			if err != nil {
				t.Error(err)
			}

			// https://developer.atlassian.com/cloud/jira/platform/modules/web-panel/
			err = p.AddWebPanel("",
				WebPanel{
					Conditions: []Conditions{
						{
							Condition: "user_is_logged_in", // https://developer.atlassian.com/cloud/jira/platform/modules/single-condition/
							Or: []Conditions{ // https://developer.atlassian.com/cloud/jira/platform/modules/composite-condition/
								{
									Condition: "jira_expression",
									Params: ConditionParams{
										Expression: "project.style == 'classic'",
									},
								}},
						},
					},
					Context:  "addon",
					Key:      "some-key",
					Location: "atl.jira.view.issue.right.context",
					Name: Name{
						Value: "Some Relevant Data",
					},
					URL: "yourpanel/path?issueId={issue.id}", // see https://developer.atlassian.com/cloud/jira/platform/context-parameters/
					/*
					   // the following are available
					   option.id, option.key, option.properties
					   issue.id, issue.key
					   project.id, project.key
					   user.id (deprecated), user.name (deprecated), user.accountId
					*/
					Weight: 10,
				})
			if err != nil {
				t.Error(err)
			}
			// https://developer.atlassian.com/cloud/jira/platform/modules/web-panel/
			err = p.AddWebPanel("jiraProjectAdminTabPanels",
				WebPanel{
					Context:  "addon",
					Key:      "some-key",
					Location: "atl.jira.view.issue.right.context",
					Name: Name{
						Value: "Some Relevant Data",
					},
					URL: "yourpanel/path?issueId={issue.id}", // see https://developer.atlassian.com/cloud/jira/platform/context-parameters/
					/*
					   // the following are available
					   option.id, option.key, option.properties
					   issue.id, issue.key
					   project.id, project.key
					   user.id (deprecated), user.name (deprecated), user.accountId
					*/
					Weight: 10,
				})
			if err != nil {
				t.Error(err)
			}
			return tCase{
				name:    "Test atlassian connect rendering",
				p:       p,
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.p
			w := &bytes.Buffer{}
			if err := p.renderAtlassianConnectJSON(w); (err != nil) != tt.wantErr {
				t.Errorf("Plugin.renderAtlassianConnectJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotW := w.String()
			abide.Assert(t, tt.name, abide.String(gotW))
		})
	}
}
