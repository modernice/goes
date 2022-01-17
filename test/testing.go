package test

//go:generate mockgen -source=testing.go -destination=./mock_test/testing.go

type TestingT interface {
	Fatal(...any)
}
