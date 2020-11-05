package storage

//    Copyright 2020 ShiftLeft Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

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
