package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/mrjones/oauth"
	"github.com/pkg/browser"
)

const urlPrefix = "https://secure.splitwise.com/api/v3.0/"

func main() {
	// TODO consider asking for these interactively in the authenticate command
	var consumerKey = flag.String("consumer-key", os.Getenv("SPLITWISE_CONSUMER_KEY"), "Contains the consumer key. Environment variable SPLITWISE_CONSUMER_KEY can be used instead.")
	var consumerSecret = flag.String("consumer-secret", os.Getenv("SPLITWISE_CONSUMER_SECRET"), "Contains the consumer secret. Environment variable SPLITWISE_CONSUMER_SECRET can be used instead.")
	flag.Parse()

	if *consumerKey == "" || *consumerSecret == "" {
		flag.Usage()
		os.Exit(2)
	}

	if len(os.Args) < 2 {
		fmt.Print("A subcommand must be invoked, subcommands available:\n\n")
		fmt.Println("test", "tests the app")
		fmt.Println("groups", "lists the groups you belong to")

		fmt.Println()
		os.Exit(2)
	}
	consumer := oauth.NewConsumer(
		*consumerKey,
		*consumerSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   urlPrefix + "get_request_token",
			AccessTokenUrl:    urlPrefix + "get_access_token",
			AuthorizeTokenUrl: "https://secure.splitwise.com/authorize",
			HttpMethod:        "POST",
			BodyHash:          true,
		},
	)

	switch os.Args[1] {
	case "test":
		err := test(consumer)
		if err != nil {
			fmt.Printf("Error authenticating: %v", err)
			os.Exit(1)
		}
	case "groups":
		accessToken, err := getAccessToken(consumer)
		if err != nil {
			fmt.Printf("failed to authenticated, gotten: %v", err)
			os.Exit(1)
		}
		response, err := consumer.Get(urlPrefix+"get_groups", nil, accessToken)
		if err != nil {
			fmt.Printf("failed to request groups, gotten: %v", err)
			os.Exit(1)
		}
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}

		// TODO decode groups properly and present content nicely
		fmt.Println(string(content))
	default:
		fmt.Println("Unknown subcommand", os.Args[1])
		os.Exit(2)
	}
}

func getAccessToken(consumer *oauth.Consumer) (*oauth.AccessToken, error) {
	// TODO check storage
	file, err := os.Open("/tmp/splitwise")
	if err == nil {
		var token = new(oauth.AccessToken)
		err = gob.NewDecoder(file).Decode(token)
		if err == nil {
			return token, nil
		} else {
			fmt.Println(err)
		}
	} else {
		fmt.Println(err)
	}

	// if not in storage authenticate
	return authenticate(consumer)
}

func authenticate(consumer *oauth.Consumer) (*oauth.AccessToken, error) {
	var verification = make(chan string)
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			verification <- r.URL.Query().Get("oauth_verifier")
			io.WriteString(w, "Token obtained successfully, you can go back to the terminal")
			// TODO disable server after returning the response
		})

		// TODO find a simple way to get a
		log.Println(http.ListenAndServe(":1234", nil)) // TODO discriminate errors
	}()
	rtoken, loginURL, err := consumer.GetRequestTokenAndUrl("http://localhost:1234")
	if err != nil {
		return nil, fmt.Errorf("error getting request token: %v", err)
	}

	browser.OpenURL(loginURL)

	accessToken, err := consumer.AuthorizeToken(rtoken, <-verification)
	if err != nil {
		return nil, err
	}

	file, err := os.Create("/tmp/splitwise")
	if err != nil {
		return nil, err
	}
	err = gob.NewEncoder(file).Encode(accessToken)

	return accessToken, err
}

func test(consumer *oauth.Consumer) error {
	accessToken, err := getAccessToken(consumer)
	if err != nil {
		return err
	}
	response, err := consumer.Get("https://secure.splitwise.com/api/v3.0/test", nil, accessToken)
	if err != nil {
		return fmt.Errorf("Error with the test: %v", err)
	}
	response.Body.Close()

	if response.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code for test: %d", response.StatusCode)
	}

	fmt.Println("Authenticated correctly")

	return nil
}
