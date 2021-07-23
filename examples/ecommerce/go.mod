module ecommerce

go 1.16

replace github.com/modernice/goes => ../../

require (
	github.com/AlecAivazis/survey/v2 v2.2.9
	github.com/alecthomas/kong v0.2.16
	github.com/google/uuid v1.2.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/modernice/goes v0.0.0-00010101000000-000000000000
	github.com/nats-io/stan.go v0.8.1
	go.mongodb.org/mongo-driver v1.5.1
	golang.org/x/sys v0.0.0-20210320140829-1e4c9ba3b0c4 // indirect
)
