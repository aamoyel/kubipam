package controller

import "time"

const (
	requeueTime      = 60 * time.Second
	domainName       = ".amoyel.fr"
	ipcidrFinalizer  = "ipcidr" + domainName
	ipclaimFinalizer = "ipclaim" + domainName
)
