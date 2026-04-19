package main

import (
	"crypto/tls"
	"fmt"
	"goFactServView/cwlog"
	"os"
	"sync"
	"time"
)

var (
	certLock    sync.RWMutex
	currentCert *tls.Certificate
)

func autoUpdateCert() {
	fullchainStat, privkeyStat := certFilesStat()

	for {
		time.Sleep(time.Minute)

		updatedFullchain, updatedPrivkey := certFilesStat()
		if updatedFullchain == nil || updatedPrivkey == nil {
			continue
		}

		if certStatChanged(fullchainStat, updatedFullchain) || certStatChanged(privkeyStat, updatedPrivkey) {
			if err := reloadCerts(); err != nil {
				cwlog.DoLog(true, "Cert reload failed: %v", err)
				continue
			}
			fullchainStat = updatedFullchain
			privkeyStat = updatedPrivkey
			cwlog.DoLog(true, "Reloaded TLS certificate.")
		}
	}
}

func reloadCerts() error {
	cert, err := tls.LoadX509KeyPair("data/certs/fullchain.pem", "data/certs/privkey.pem")
	if err != nil {
		return fmt.Errorf("error loading TLS key pair data/certs/(fullchain.pem, privkey.pem): %w", err)
	}

	certLock.Lock()
	currentCert = &cert
	certLock.Unlock()
	return nil
}

func loadCerts() error {
	if err := reloadCerts(); err != nil {
		return err
	}
	cwlog.DoLog(true, "Loaded certs.")
	return nil
}

func getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certLock.RLock()
	defer certLock.RUnlock()

	if currentCert == nil {
		return nil, fmt.Errorf("TLS certificate is not loaded")
	}

	return currentCert, nil
}

func certFilesStat() (*os.FileInfo, *os.FileInfo) {
	fullchainStat, err := os.Stat("data/certs/fullchain.pem")
	if err != nil {
		return nil, nil
	}

	privkeyStat, err := os.Stat("data/certs/privkey.pem")
	if err != nil {
		return nil, nil
	}

	return &fullchainStat, &privkeyStat
}

func certStatChanged(previous, current *os.FileInfo) bool {
	if previous == nil || current == nil {
		return previous != current
	}

	return (*previous).Size() != (*current).Size() || (*previous).ModTime() != (*current).ModTime()
}
