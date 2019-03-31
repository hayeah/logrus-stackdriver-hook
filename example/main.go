package main

import (
	"cloud.google.com/go/errorreporting"
	"cloud.google.com/go/logging"
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"log"

	"github.com/hayeah/logrus-stackdriver-hook"
	"github.com/pkg/errors"
)

func reportErr() error {
	ctx := context.Background()
	projectID, ok := os.LookupEnv("PROJECT_ID")
	if !ok {
		return errors.New("PROJECT_ID is not set")
	}

	cred := option.WithCredentialsFile("credentials.json")

	client, err := logging.NewClient(ctx, projectID, cred)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	logger := client.Logger("sdhook")

	errclient, err := errorreporting.NewClient(ctx, projectID, errorreporting.Config{
		ServiceName: "sdhook",
		OnError: func(err error) {
			log.Printf("Could not log error: %v", err)
		},
	}, cred)
	defer errclient.Close()

	if err != nil {
		return err
	}

	logrus.AddHook(hook.NewLog(logger))
	logrus.AddHook(hook.NewErrorReport(errclient))

	newerr := errors.New("do foo")
	logrus.WithField("user", "howard").Errorln("send user err:", errors.Wrap(newerr, "do bar"))

	logrus.Warnln("a warning will be sent")
	logrus.Infoln("an info will be sent")

	logrus.Debugln("this is a debug line, should not be sent to logging service")

	return nil
}

func main() {
	err := reportErr()
	if err != nil {
		log.Fatalln(err)
	}
}
