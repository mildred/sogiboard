package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jbenet/go-base58"
	"github.com/mildred/sogiboard/crypt"
	"github.com/mildred/sogiboard/server"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
	"os"
)

const (
	DriveReadonlyScope         = "https://www.googleapis.com/auth/drive.readonly"
	DriveMetadataReadonlyScope = "https://www.googleapis.com/auth/drive.metadata.readonly"
)

var (
	config = oauth2.Config{
		ClientID:     "",
		ClientSecret: "",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:      []string{DriveMetadataReadonlyScope, DriveReadonlyScope},
	}
)

func Getenv(varname string, defval ...string) string {
	res := os.Getenv(varname)
	for res == "" && len(defval) > 0 {
		res = defval[0]
		defval = defval[1:]
	}
	return res
}

func main() {
	gen_token := flag.Bool("gen-token", false, "Generate token from the authorization code")
	client_token := flag.String("client-token", Getenv("CLIENT_TOKEN"), "OAuth Client token as JSON")
	client_id := flag.String("client-id", Getenv("CLIENT_ID"), "OAuth Client ID")
	client_secret := flag.String("client-secret", Getenv("CLIENT_SECRET"), "OAuth Client secret")
	listen := flag.String("listen", ":"+Getenv("PORT", "8080"), "Listen address and port")
	secret_key := flag.String("secret-key", Getenv("SECRET_KEY"), "Secret key for URL parameters encryption")
	gen_secret_key := flag.Bool("gen-secret-key", false, "Generate secret key")
	flag.Parse()

	if client_id != nil && *client_id != "" {
		config.ClientID = *client_id
	}
	if client_secret != nil && *client_secret != "" {
		config.ClientSecret = *client_secret
	}

	if *gen_token {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		fmt.Printf("Go to the following address and paste the code back here.\n\n%s\n\n", authURL)

		var code string
		if _, err := fmt.Scan(&code); err != nil {
			log.Fatalf("Unable to read authorization code %v", err)
		}

		tok, err := config.Exchange(oauth2.NoContext, code)
		if err != nil {
			log.Fatalf("Unable to retrieve token from web %v", err)
		}

		data, err := json.Marshal(tok)
		if err != nil {
			log.Fatalf("Unable to marshal the token %v", err)
		}

		fmt.Printf("\nCLIENT_TOKEN='%s'\n", data)
		return
	}

	if *gen_secret_key {
		sk, err := crypt.NewSecretKey()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		sktext := base58.Encode(sk)
		fmt.Printf("SECRET_KEY=\"%s\"\n", sktext)
		return
	}

	if client_token == nil || *client_token == "" {
		log.Fatal("Missing CLIENT_TOKEN. You can generate using the interactive command line option --gen-token")
	}

	var tok oauth2.Token
	err := json.Unmarshal([]byte(*client_token), &tok)
	if err != nil {
		log.Fatalf("Unable to marshal the token %v", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, &tok)

	err = server.Init(*listen, client, *secret_key)
	if err != nil {
		log.Fatal(err)
	}
}

type GidOption string

func (gid GidOption) Get() (string, string) { return "gid", string(gid) }
