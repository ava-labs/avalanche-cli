package models

type Host struct {
	NodeID            string
	IP                string
	SshUser           string
	SshPrivateKeyPath string
	SshCommonArgs     string
}
