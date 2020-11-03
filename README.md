# atlassian-connect-go

This is a set of tools to create atlassian connect jira plugins in go

# Storage

Storage provides an interface `storage.Store` that users must implement to be able to persist jira plugin information.

It also provides `storage.JiraInstallInformation` which holds all the information provided by jira upon installation.

# Handling

Most of the tooling live here, by properly instantiating and configuring a `handling.Plugin` you can have your
full jira cloud plug-in working in no time.

```go
p = handling.NewPlugin("my jira plugin", "A nice jira plug in, that plugs the ins of jira cloud",
"com.yourcompany.something.jira.plugin", "https://wherever.you.serve.this", 
"/relative/path/to/your/endpoints/if/any",
[]string{"READ", "WRITE"}) // as defined in https://developer.atlassian.com/cloud/jira/platform/scopes/

err := p.AddLifeCycleEvent(handling.LCEInstall, "/install", handleInstallFunc)
if err != nil {
    //...
}

err = p.AddWebHook("jira:issue_updated", "/issue_updated", handleIssueUpdated)
if err != nil {
    //...
}

err = p.AddJiraIssueField(handling.JiraIssueField{
        Description: handling.Description{Value:"A more detailed description"},
	    Key          : "A_Field",
	    Name         :"A Fancy Field",
	    Type         :"text"}) // https://developer.atlassian.com/cloud/jira/platform/modules/issue-field/
if err != nil {
    //...
}

// https://developer.atlassian.com/cloud/jira/platform/modules/web-panel/
err = p.AddWebPanel("",
    handling.WebPanel{
		Conditions: []handling.Conditions{
            {
                Condition: "user_is_logged_in", // https://developer.atlassian.com/cloud/jira/platform/modules/single-condition/
                Or: [ // https://developer.atlassian.com/cloud/jira/platform/modules/composite-condition/
                 handling.Conditions{
                    Condition:"jira_expression",
                    Params: handling.ConditionParams{
                        Expression:"project.style == 'classic'",
                    },
                }],
            },
        },
		Context:    "addon",
		Key:        "some-key",
		Location:   "atl.jira.view.issue.right.context",
		Name: Name{
			Value: "Some Relevant Data",
		},
        URL:    "yourpanel/path?issueId={issue.id}", // see https://developer.atlassian.com/cloud/jira/platform/context-parameters/
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
    //...
}

// and there you go, acRouter will serve:
// * atlassian-connect.json with the info we added when instancing and all the extra config
// * will listen for the passed in lifecycle endpoints and webhooks
// if you want for it to also handle the routes for panels you will need to add the paths yourself
acRouter := p.Router(yourRouterIfAny)

```

# Apicommunication

Provides an `apicommunication.HostClient` and a series of other helpers such as `apicommunication.HostClient{}.Do`
and `apicommunication.HostClient{}.DoWithTarget` which can be instantiated using `storage.JiraInstallInformation` 
and are modeled close to the official Atlassian Connect typescript module.

A `apicommunication.ValidateRequest` function is provided that will try to validate an incoming request from jira.

Additionally a large sets of types generated from jira's own documentation are provided for ease of use of `DoWithTarget`

Peruse the package to find a few extra helpers that might or might not be useful for your case.