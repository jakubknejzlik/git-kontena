package kontena

import (
	"fmt"
	"log"

	"github.com/inloop/goclitools"
	"github.com/jakubknejzlik/kontena-git-cli/model"
	"github.com/urfave/cli"
)

// CertificateInstall ...
func (c *Client) CertificateInstall(cert model.Certificate) error {
	return c.CertificateInstallInGrid(c.CurrentGrid().Name, cert)
}

// CertificateInstallInGrid ...
func (c *Client) CertificateInstallInGrid(grid string, cert model.Certificate) error {
	goclitools.Log("installing certificate", cert.Domain, "in grid", grid)
	if cert.Bundle != "" {
		return c.DeployCertificateInGrid(grid, cert, cert.Bundle)
	}

	if cert.Bundle == "" && cert.LetsEncrypt {
		return c.issueLECertificateInGrid(grid, cert)
	}

	return cli.NewExitError(fmt.Sprintf(`certificate %s is not marked as letsencrypt and doesn't contain bundle`, cert.Domain), 1)
}

// DeployCertificate ...
func (c *Client) DeployCertificate(cert model.Certificate, bundle string) error {
	goclitools.Log("writing certificate", cert.SecretName())
	return c.SecretWrite(cert.SecretName(), bundle)
}

// DeployCertificateInGrid ...
func (c *Client) DeployCertificateInGrid(grid string, cert model.Certificate, bundle string) error {
	goclitools.Log("writing certificate", cert.SecretName(), "grid", grid)
	return c.SecretWriteToGrid(grid, cert.SecretName(), bundle)
}

func (c *Client) issueLECertificateInGrid(grid string, cert model.Certificate) error {
	service := model.KontenaService{
		Environment: []string{
			"KONTENA_LB_VIRTUAL_HOSTS=" + cert.Domain,
			"KONTENA_LB_VIRTUAL_PATH=/.well-known/acme-challenge",
		},
		Links: []string{
			"core/internet_lb",
		},
		Image: "jakubknejzlik/acme.sh-nginx",
	}
	serviceName := "acme-challenge"

	if err := c.removeAcmeServiceFromGrid(grid); err != nil {
		return err
	}

	if err := c.ServiceCreateInGrid(grid, serviceName, service); err != nil {
		return err
	}

	if err := c.ServiceDeployInGrid(grid, serviceName); err != nil {
		return err
	}

	goclitools.Log("issuing certificate")
	issueCmd := fmt.Sprintf(`/issue.sh %s`, cert.Domain)
	if data, err := c.ServiceExecInGrid(grid, serviceName, issueCmd); err != nil {
		log.Println(err, string(data))
		// return err
	}

	goclitools.Log("fetching certificate")
	loadCertCmd := fmt.Sprintf(`cat /root/.acme.sh/%s/fullchain.cer /root/.acme.sh/%s/%s.key`, cert.Domain, cert.Domain, cert.Domain)
	if data, err := c.ServiceExecInGrid(grid, serviceName, loadCertCmd); err == nil {
		c.DeployCertificate(cert, string(data))
	} else {
		return err
	}

	return c.removeAcmeServiceFromGrid(grid)
}

func (c *Client) removeAcmeServiceFromGrid(grid string) error {
	exists, err := c.ServiceExistsInGrid(grid, "", "acme-challenge")
	if err != nil {
		return err
	}
	if exists == false {
		return nil
	}
	if err := c.GridUse(grid); err != nil {
		return err
	}
	goclitools.Log("removing acme-challenge service")
	return c.ServiceRemove("acme-challenge")
}

// CurrentCertificateSecrets ...
func (c *Client) CurrentCertificateSecrets() ([]model.Secret, error) {
	certs := []model.Secret{}
	secrets, secretsErr := c.SecretList()

	if secretsErr != nil {
		return certs, secretsErr
	}

	for _, secret := range secrets {
		if secret.IsCertificate() {
			certs = append(certs, secret)
		}
	}

	return certs, nil
}

// CurrentCertificateSecretsInGrid ...
func (c *Client) CurrentCertificateSecretsInGrid(grid string) ([]model.Secret, error) {
	certs := []model.Secret{}
	secrets, secretsErr := c.SecretListInGrid(grid)

	if secretsErr != nil {
		return certs, secretsErr
	}

	for _, secret := range secrets {
		if secret.IsCertificate() {
			certs = append(certs, secret)
		}
	}

	return certs, nil
}
