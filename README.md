# atlassian-connect-go

This repo contains a set of tools you can use to create Jira plugins using the
Atlassian Connect framework. It is written in Go.

## Storage

Storage includes an interface called `storage.Store` that users must
implement to persist Jira plugin information.

Storage also includes `storage.JiraInstallInformation`, which handles
the information provided by Jira upon installation.

## Handling

Handling contains most of the tooling. It includes what you need to
instantiate and configure `handling.Plugin`, which has most of
what you will need to create your Jira Cloud plugin.

The handler function is `handling.JiraHandleFunc`, which is wrapped
in `http.HandlerFunc` for validation if possible.

We also provide `Plugin.VerifiedHandleFunc` and `Plugin.UnverifiedHandleFunc`,
which allows you to create your own `http.HandlerFunc` with JWT validation.

```go
p = handling.NewPlugin(
        "my jira plugin", // human-readable name for your plugin
        "A nice jira plug in, that plugs the ins of jira cloud", // long description
        "com.yourcompany.something.jira.plugin", // unique internal key
        "https://wherever.you.serve.this", // this plugin's URL
        "/relative/path/to/your/endpoints/if/any", // a relative path if this is not served from the base
        // bear in mind it will be automatically added to your paths in the following steps
        []string{"READ", "WRITE"}) // defined in https://developer.atlassian.com/cloud/jira/platform/scopes/

err := p.AddLifecycleEvent(handling.LCInstalled, "/install", handleInstallFunc)
if err != nil {
    //...
}

err = p.AddWebhook("jira:issue_updated", "/issue_updated", handleIssueUpdated)
if err != nil {
    //...
}

err = p.AddJiraIssueField(handling.JiraIssueFields{
    Description: handling.Description{Value: "A more detailed description"},
    Key:         "A_Field",
    Name:        handling.Name{Value: "A Fancy Field"},
    Type:        "text"}) // https://developer.atlassian.com/cloud/jira/platform/modules/issue-field/
if err != nil {
    //...
}

// https://developer.atlassian.com/cloud/jira/platform/modules/web-panel/
err = p.AddWebPanel("",
    handling.WebPanel{
        Conditions: []handling.Conditions{
            {
                Condition: "user_is_logged_in", // https://developer.atlassian.com/cloud/jira/platform/modules/single-condition/
                Or: []handlingConditions{ // https://developer.atlassian.com/cloud/jira/platform/modules/composite-condition/
                    handling.Conditions{
                        Condition: "jira_expression",
                        Params: handling.ConditionParams{
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
    //...
}

// acRouter serves:
// * atlassian-connect.json with the info we added when instantiating the plugin and the extra config
// * will listen for the lifecycle endpoints and webhooks that's passed in
// if you want it to handle the routes for panels, you will need to add the paths yourself
acRouter := p.Router(yourRouterIfAny)

```

## apicommunication

The **apicommuncation** folder provides `apicommunication.HostClient`,
as well as a series of helpers such as `apicommunication.HostClient{}.Do`
and `apicommunication.HostClient{}.DoWithTarget`. The helpers are modeled
on the official Atlassian Connect TypeScript module and can be instantiated
using `storage.JiraInstallInformation`.

We've provided an `apicommunication.ValidateRequest` function that will try
to validate an incoming request from Jira.

Additionally, we provide you with a large set of types generated using
information from Jira's documentation to make it easy to use `DoWithTarget`.

There are a few extra helpers that you may find helpful for your use case.
