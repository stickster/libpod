package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/docker"
	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/libpod/libpod/common"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	loginFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "password, p",
			Usage: "Password for registry",
		},
		cli.StringFlag{
			Name:  "username, u",
			Usage: "Username for registry",
		},
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override. ",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "Pathname of a directory containing TLS certificates and keys used to connect to the registry",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "Require HTTPS and verify certificates when contacting registries (default: true)",
		},
	}
	loginDescription = "Login to a container registry on a specified server."
	loginCommand     = cli.Command{
		Name:         "login",
		Usage:        "Login to a container registry",
		Description:  loginDescription,
		Flags:        sortFlags(loginFlags),
		Action:       loginCmd,
		ArgsUsage:    "REGISTRY",
		OnUsageError: usageErrorHandler,
	}
)

// loginCmd uses the authentication package to store a user's authenticated credentials
// in an auth.json file for future use
func loginCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) > 1 {
		return errors.Errorf("too many arguments, login takes only 1 argument")
	}
	if len(args) == 0 {
		return errors.Errorf("registry must be given")
	}
	var server string
	if len(args) == 1 {
		server = args[0]
	}
	authfile := getAuthFile(c.String("authfile"))

	sc := common.GetSystemContext("", authfile, false)

	// username of user logged in to server (if one exists)
	userFromAuthFile, err := config.GetUserLoggedIn(sc, server)
	if err != nil {
		return errors.Wrapf(err, "error getting logged-in user")
	}
	username, password, err := getUserAndPass(c.String("username"), c.String("password"), userFromAuthFile)
	if err != nil {
		return errors.Wrapf(err, "error getting username and password")
	}
	sc.DockerInsecureSkipTLSVerify = !c.BoolT("tls-verify")
	if c.String("cert-dir") != "" {
		sc.DockerCertPath = c.String("cert-dir")
	}

	if err = docker.CheckAuth(context.TODO(), sc, username, password, server); err == nil {
		if err := config.SetAuthentication(sc, server, username, password); err != nil {
			return err
		}
	}
	switch err {
	case nil:
		fmt.Println("Login Succeeded!")
		return nil
	case docker.ErrUnauthorizedForCredentials:
		return errors.Errorf("error logging into %q: invalid username/password\n", server)
	default:
		return errors.Wrapf(err, "error authenticating creds for %q", server)
	}
}

// getUserAndPass gets the username and password from STDIN if not given
// using the -u and -p flags
func getUserAndPass(username, password, userFromAuthFile string) (string, string, error) {
	var err error
	reader := bufio.NewReader(os.Stdin)
	if username == "" {
		if userFromAuthFile != "" {
			fmt.Printf("Username (%s): ", userFromAuthFile)
		} else {
			fmt.Print("Username: ")
		}
		username, err = reader.ReadString('\n')
		if err != nil {
			return "", "", errors.Wrapf(err, "error reading username")
		}
	}
	if password == "" {
		fmt.Print("Password: ")
		pass, err := terminal.ReadPassword(0)
		if err != nil {
			return "", "", errors.Wrapf(err, "error reading password")
		}
		password = string(pass)
		fmt.Println()
	}
	return strings.TrimSpace(username), password, err
}
