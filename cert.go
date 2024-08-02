package main

import (
	"crypto/tls"
	"goFactServView/cwlog"
	"os"
	"time"
)

func autoUpdateCert() {
	for {
		time.Sleep(time.Minute)

		filePath := "data/certs/fullchain.pem"
		initialStat, erra := os.Stat(filePath)

		if erra != nil {
			continue
		}

		for initialStat != nil {
			time.Sleep(time.Minute)

			stat, errb := os.Stat(filePath)
			if errb != nil {
				break
			}

			if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
				cwlog.DoLog(true, "Cert updated, closing.")
				time.Sleep(time.Second * 5)
				os.Exit(0)
				break
			}
		}

	}
}

func loadCerts() tls.Certificate {
	/* Load certificates */
	cert, err := tls.LoadX509KeyPair("data/certs/fullchain.pem", "data/certs/privkey.pem")
	if err != nil {
		cwlog.DoLog(true, "Error loading TLS key pair: %v data/certs/(fullchain.pem, privkey.pem)", err)
		return tls.Certificate{}
	}
	cwlog.DoLog(true, "Loaded certs.")
	return cert
}
