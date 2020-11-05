package handling

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
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

func (f *fakeStore) SaveJiraInstallInformation(j *storage.JiraInstallInformation) error {
	f.j = j
	return nil
}

func (f *fakeStore) JiraInstallInformation(clientKey string) (*storage.JiraInstallInformation, error) {
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

var fakeHandleFunc = func(jii *storage.JiraInstallInformation, s storage.Store, w http.ResponseWriter, r *http.Request) {}

func newPlugin(t *testing.T, handleFunc JiraHandleFunc) *Plugin {
	l := adaptLogger(t)
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
	err := p.AddLifecycleEvent(LCInstalled, "/installed", handleFunc)
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
	return p
}

func TestPlugin_renderAtlassianConnectJSON(t *testing.T) {

	tests := []tCase{
		func() tCase {
			p := newPlugin(t, fakeHandleFunc)
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

func TestPlugin_servesAtlasianJSON(t *testing.T) {

	tests := []tCase{
		func() tCase {
			p := newPlugin(t, fakeHandleFunc)
			return tCase{
				name:    "Test atlassian connect serving",
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
			ts := httptest.NewServer(p.Router(nil))
			defer ts.Close() // if you add more cases beware, these wont be closed until the whole test is exited
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/path/to/api/atlassian-connect.json", nil)
			if err != nil {
				t.Fatal(err)
			}

			client := &http.Client{}
			res, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			b, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}

			abide.Assert(t, "serving atlassian connect", abide.String(string(b)))

		})
	}
}

func TestPlugin_servesInstall(t *testing.T) {
	var sentJII storage.JiraInstallInformation
	hf := func(jii *storage.JiraInstallInformation, store storage.Store, w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		// jii is nil here
		err := json.NewDecoder(r.Body).Decode(&sentJII)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	initialJii := &storage.JiraInstallInformation{
		UserAccount:    "uaccount",
		Key:            "ukey",
		ClientKey:      "ckey",
		OauthClientID:  "sdadsadsadas",
		PublicKey:      "kasdhaskdjhaksjhdka",
		SharedSecret:   "kiasjhdkajhdkajshd",
		ServerVersion:  "1",
		PluginsVersion: "2",
		BaseURL:        "http://www.atlassian.net",
		ProductType:    "jira",
		Description:    "a jira plugin",
		EventType:      "installed",
	}

	t.Run("serves install", func(t *testing.T) {
		p := newPlugin(t, hf)
		w := &bytes.Buffer{}
		if err := p.renderAtlassianConnectJSON(w); err != nil {
			t.Fatal(err)
		}
		ts := httptest.NewServer(p.Router(nil))
		defer ts.Close()
		bodyBytes, err := json.MarshalIndent(initialJii, "", "    ")
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/path/to/api/installed", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("%#v", err)
		}
		if res.StatusCode != 200 {
			t.Logf("server responded %d", res.StatusCode)
			t.FailNow()
		}

		marshalledSentJii, err := json.MarshalIndent(sentJII, "", "    ")
		if err != nil {
			t.Fatal(err)
		}
		if string(marshalledSentJii) != string(bodyBytes) {
			t.Logf("%s\nis different from\n%s", marshalledSentJii, bodyBytes)
			t.FailNow()
		}

	})

}
