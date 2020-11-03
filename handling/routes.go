package handling

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

// NewRoutePath returns a new instance of RoutePath with the correct fields filled.
func NewRoutePath(path string, args map[string]string) RoutePath {
	return RoutePath{path: path, keys: args}
}

// RoutePath wraps in a crude way some url components so we can distinguish between the kind
// of URL that we pass to jira and the one we pass to the router.
type RoutePath struct {
	path string
	keys map[string]string
}

func (r *RoutePath) url() string {
	if len(r.keys) == 0 {
		return r.path
	}
	kvs := make([]string, 0, len(r.keys))
	for k, v := range r.keys {
		kvs = append(kvs, k+"="+v)
	}
	return r.path + "?" + strings.Join(kvs, "&")
}

// Plugin represents an atlassian connect plugin instance
type Plugin struct {
	ac        *AtlassianConnect
	logger    *log.Logger
	baseRoute string

	jiraIssueFields map[string]JiraIssueFields

	lifecycle       map[LifeCycleEvents]http.HandlerFunc
	lifecycleRoutes map[LifeCycleEvents]string

	webhooks      map[string]http.HandlerFunc
	webhookRoutes map[string]RoutePath

	arbitraryWebPanels map[string][]WebPanel
}

// Router returns a router for the handled cases in this plugin
// panel handlers are not covered here so if you want them you can add them to the returned router.
// The returned router is based on the passed one if provided.
func (p *Plugin) Router(r *mux.Router) *mux.Router {
	var newRouter *mux.Router
	if r == nil {
		newRouter = mux.NewRouter()
	}
	if p.baseRoute != "" {
		newRouter = r.PathPrefix(p.baseRoute).Subrouter()
	} else {
		newRouter = r
	}
	newRouter.Methods(http.MethodGet).Path("atlassian-connect.json").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "application/json")
		if err := json.NewEncoder(w).Encode(p.ac); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if p.logger != nil {
				p.logger.Printf("ERROR: %v", err)
			}
		}
	})
	for event, handler := range p.lifecycle {
		newRouter.Methods(http.MethodGet, http.MethodPost).Path(p.lifecycleRoutes[event]).HandlerFunc(handler)
	}
	for hook, handler := range p.webhooks {
		newRouter.Methods(http.MethodGet, http.MethodPost).Path(p.webhookRoutes[hook].path).HandlerFunc(handler)
	}

	return newRouter
}

var defaultPluginAuthentication = Authentication{
	Type: "jwt",
}

// LifeCycleEvents are the possible events in the plugin lifecycle we can receive from JIRA.
type LifeCycleEvents string

const (
	// LCInstalled is invoked when the plugin is [re]installed
	LCInstalled LifeCycleEvents = "installled"
	// LCUnInstalled is invoked when the plugin is un installed
	LCUnInstalled LifeCycleEvents = "uninstallled"
	// LCEnabled is invoked when the plugin is enabled
	LCEnabled LifeCycleEvents = "enabled"
	// LCDisabled is invoked when the plugin is disabled
	LCDisabled LifeCycleEvents = "disabled"
)

// AddWebPanel will add the passed webpanel to to the pased container and fail if already present.
// Possible panel containers are documented in https://developer.atlassian.com/cloud/jira/platform/about-jira-modules/
// as locations.
func (p *Plugin) AddWebPanel(panelContainer string, wp WebPanel) error {
	ewp, exists := p.arbitraryWebPanels[panelContainer]
	if exists {
		for _, v := range ewp {
			if v.Key == wp.Key {
				return fmt.Errorf("panel %s is already defined in container %s", wp.Key, panelContainer)
			}
		}
	}
	return p.UpdateWebPanel(panelContainer, wp)
}

// UpdateWebPanel will add the passed webpanel to to the pased container, if there is one in place
// it will be replaced.
func (p *Plugin) UpdateWebPanel(panelContainer string, wp WebPanel) error {
	ewp, exists := p.arbitraryWebPanels[panelContainer]
	if !exists {
		p.arbitraryWebPanels[panelContainer] = []WebPanel{wp}
		return nil
	}
	ewp = append(ewp, wp)
	p.arbitraryWebPanels[panelContainer] = ewp
	for k, v := range p.arbitraryWebPanels {
		p.ac.Modules[k] = v
	}
	return nil
}

// AddJiraIssueField will add the passed issue field to the issue fields section, it will fail if
// it is already present.
// Details on the values of an JiraIssueField can be found at
// https://developer.atlassian.com/cloud/jira/platform/modules/issue-field/
func (p *Plugin) AddJiraIssueField(f JiraIssueFields) error {
	if _, exists := p.jiraIssueFields[f.Key]; exists {
		return fmt.Errorf("%s is already registered", f.Key)
	}
	return p.UpdateJiraIssueField(f)
}

const jiraIssueFieldsKey = "jiraIssueFields"

// UpdateJiraIssueField will add the passed issue field to the issue fields section, it will replace
// it if already present.
func (p *Plugin) UpdateJiraIssueField(f JiraIssueFields) error {
	p.jiraIssueFields[f.Key] = f
	jIFields := make([]JiraIssueFields, 0, len(p.jiraIssueFields))
	for k := range p.jiraIssueFields {
		jIFields = append(jIFields, p.jiraIssueFields[k])
	}
	p.ac.Modules[jiraIssueFieldsKey] = jIFields
	return nil
}

// AddWebhook will add a webhook to a given jira event (of the form jira:issue_updated) or fail if
// already present, a more exhaustive list is available in jira documentation at
// https://developer.atlassian.com/cloud/jira/platform/webhooks/
func (p *Plugin) AddWebhook(event string, route RoutePath, f http.HandlerFunc) error {
	if _, exists := p.webhooks[event]; exists {
		return fmt.Errorf("%s event is already being handled", event)
	}
	return p.UpdateWebhook(event, route, f)
}

const webhooksKey = "webhooks"

// UpdateWebhook will add a webhook to a given jira event, if already present it will be replaced.
func (p *Plugin) UpdateWebhook(event string, route RoutePath, f http.HandlerFunc) error {
	p.webhooks[event] = f
	p.webhookRoutes[event] = route
	webhooks := []Webhooks{}
	for k, v := range p.webhookRoutes {
		webhooks = append(webhooks, Webhooks{
			Event: k,
			URL:   v.url(),
		})
	}
	// since modules admits a great deal of arbitrary modules we just do it like a map to interface
	p.ac.Modules[webhooksKey] = webhooks
	return nil
}

// AddLifecycleEvent adds a handler for a given life cycle event, if already present it will fail.
func (p *Plugin) AddLifecycleEvent(lce LifeCycleEvents, route string, f http.HandlerFunc) error {
	if _, exists := p.lifecycle[lce]; exists {
		return fmt.Errorf("%s is already registered for this plugin", lce)
	}

	return p.UpdateLifecycleEvent(lce, route, f)
}

// UpdateLifecycleEvent adds a handler for a given life cycle event, if already present it will replace it.
func (p *Plugin) UpdateLifecycleEvent(lce LifeCycleEvents, route string, f http.HandlerFunc) error {
	p.lifecycle[lce] = f
	p.lifecycleRoutes[lce] = route
	lc := Lifecycle{}
	for k, v := range p.lifecycleRoutes {
		eventPath := path.Join(p.baseRoute, v)
		switch k {
		case LCInstalled:
			lc.Installed = eventPath
		case LCUnInstalled:
			lc.UnInstalled = eventPath
		case LCEnabled:
			lc.Enabled = eventPath
		case LCDisabled:
			lc.Disabled = eventPath
		}
	}
	p.ac.Lifecycle = lc
	return nil
}

// NewPlugin will create a new Plugin instance, as it is it will not be enough, you should add the
// necesary lifecycle events, webhooks, etc using the provided methods then obtain the Router handling
// all the events by invoking Router().
func NewPlugin(name, description, key, baseURL, baseRoute string,
	scopes []string, vendor Vendor) *Plugin {
	ac := &AtlassianConnect{
		Authentication: defaultPluginAuthentication,
		BaseURL:        baseURL,
		Description:    description,
		Key:            key,
		Name:           name,
		Scopes:         scopes,
		Vendor:         vendor,
	}

	return &Plugin{
		ac:                 ac,
		baseRoute:          "",
		jiraIssueFields:    map[string]JiraIssueFields{},
		lifecycle:          map[LifeCycleEvents]http.HandlerFunc{},
		lifecycleRoutes:    map[LifeCycleEvents]string{},
		webhooks:           map[string]http.HandlerFunc{},
		webhookRoutes:      map[string]RoutePath{},
		arbitraryWebPanels: map[string][]WebPanel{},
	}
}
