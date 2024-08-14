package controllers

import "time"

const (
	requeueTime      = 60 * time.Second
	domainName       = ".didactiklabs.io"
	ipcidrFinalizer  = "ipcidr" + domainName
	ipclaimFinalizer = "ipclaim" + domainName
)
