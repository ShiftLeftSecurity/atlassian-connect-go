package storage

// JiraInstallInformation is the payload sent by JIRA to the /install endpoint
type JiraInstallInformation struct {
	UserAccount    string `json:"-"`
	Key            string `json:"key"`
	ClientKey      string `json:"clientKey"`
	OauthClientID  string `json:"oauthClientID"`
	PublicKey      string `json:"publicKey"`
	SharedSecret   string `json:"sharedSecret"`
	ServerVersion  string `json:"serverVersion"`
	PluginsVersion string `json:"pluginsVersion"`
	BaseURL        string `json:"baseURL"`
	ProductType    string `json:"productType"`
	Description    string `json:"description"`
	EventType      string `json:"eventType"`
}

// Store should be implemented to allow storage of the necessary jira information.
// all methods should be idempotent.
type Store interface {
	SaveJiraIntallInformation(*JiraInstallInformation) error
	JiraIntallInformation(clientKey string) (*JiraInstallInformation, error)
}
