package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/mozilla/tls-observatory/certificate"
	"github.com/mozilla/tls-observatory/database"
)

func main() {
	db, err := database.RegisterConnection(
		os.Getenv("TLSOBS_POSTGRESDB"),
		os.Getenv("TLSOBS_POSTGRESUSER"),
		os.Getenv("TLSOBS_POSTGRESPASS"),
		os.Getenv("TLSOBS_POSTGRES"),
		"require")
	defer db.Close()
	if err != nil {
		panic(err)
	}
	batch := 0
	for {
		fmt.Printf("\nProcessing batch %d to %d: ", batch, batch+100)
		rows, err := db.Query(`SELECT id, raw_cert
					FROM certificates
					WHERE x509_certificatepolicies IS NULL OR x509_certificatepolicies='null'
					   OR permitted_names IS NULL OR permitted_names='null'
					ORDER BY id ASC
					LIMIT 100`)
		if rows != nil {
			defer rows.Close()
		}
		if err != nil {
			panic(fmt.Errorf("Error while retrieving certs: '%v'", err))
		}
		i := 0
		for rows.Next() {
			i++
			var raw string
			var id int64
			err = rows.Scan(&id, &raw)
			if err != nil {
				fmt.Println("error while parsing cert", id, ":", err)
				continue
			}
			certdata, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				fmt.Println("error decoding base64 of cert", id, ":", err)
				continue
			}
			c, err := x509.ParseCertificate(certdata)
			if err != nil {
				fmt.Println("error while x509 parsing cert", id, ":", err)
				continue
			}
			var valInfo certificate.ValidationInfo
			cert := certificate.CertToStored(c, "", "", "", "", &valInfo)
			policies, err := json.Marshal(cert.X509v3Extensions.PolicyIdentifiers)
			if err != nil {
				log.Printf("error while marshalling policies for cert %d: %v", id, err)
				continue
			}
			permittednames, err := json.Marshal(cert.X509v3Extensions.PermittedNames)
			if err != nil {
				log.Printf("error while marshalling permitted names for cert %d: %v", id, err)
				continue
			}
			log.Printf("id=%d, subject=%s, policies=%s, permittednames=%s", id, cert.Subject.String(), policies, permittednames)
			_, err = db.Exec(`UPDATE certificates
						SET x509_certificatepolicies=$1,
						    permitted_names=$2,
						    is_name_constrained=$3
						WHERE id=$4`,
				policies,
				permittednames,
				cert.X509v3Extensions.IsNameConstrained,
				id)
			if err != nil {
				fmt.Println("error while updating cert", id, "in database:", err)
			}
			fmt.Printf(".")
		}
		if i == 0 {
			fmt.Println("done!")
			break
		}
		//offset += limit
		batch += 100
	}
}
