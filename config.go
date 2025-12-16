package main

type Config struct {
	StorageRoot    string
	URL            string
	ClientID       string
	ClientSecret   string
	AuthServerPort uint
	Scopes         []string
}
