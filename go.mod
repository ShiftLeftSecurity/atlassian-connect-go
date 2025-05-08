module github.com/ShiftLeftSecurity/atlassian-connect-go

go 1.24

require (
	github.com/andygrunwald/go-jira v1.16.0
	github.com/beme/abide v0.0.0-20190723115211-635a09831760
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/gorilla/mux v1.8.0
	github.com/pkg/errors v0.9.1
	golang.org/x/oauth2 v0.30.0
)

require (
	github.com/fatih/structs v1.1.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/nsf/jsondiff v0.0.0-20210926074059-1e845ec5d249 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/trivago/tgo v1.0.7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/beme/abide => github.com/ShiftLeftSecurity/abide v0.6.1
