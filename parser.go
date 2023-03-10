package awsparser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/igorrendulic/couchdb-email-aws-parse/sns"
	"github.com/igorrendulic/couchdb-experiment/email/mime/parser"
)

type AwsParser struct {
}

func NewAwsParser() parser.Parser {
	return &AwsParser{}
}

func (p *AwsParser) Parse(message []byte) (*parser.MailReceived, error) {
	var notificationPayload sns.Payload
	err := json.Unmarshal(message, &notificationPayload)
	if err != nil {
		return nil, err
	}

	verifyErr := notificationPayload.VerifyPayload()
	if verifyErr != nil {
		return nil, verifyErr
	}

	ts, tsErr := time.Parse(time.RFC3339, notificationPayload.Timestamp)
	if tsErr != nil {
		return nil, tsErr
	}

	if notificationPayload.Type == "SubscriptionConfirmation" {
		_, err := notificationPayload.Subscribe()
		if err != nil {
			return nil, err
		}
		return &parser.MailReceived{
			NotificationType: notificationPayload.Type,
			Timestamp:        ts.UnixMilli(),
		}, nil
	}
	if notificationPayload.Type == "Notification" {
		// TODO: Implement
		message := notificationPayload.Message
		var messageJSON MessageJSON
		err := json.Unmarshal([]byte(message), &messageJSON)
		if err != nil {
			return nil, err
		}
		if messageJSON.NotificationType == "Received" {
			mail := messageJSON.Mail
			receipt := messageJSON.Receipt

			fmt.Printf("Mail: %+v\n", mail)
			fmt.Printf("Receipt: %+v\n", receipt)
		}
	}

	mimeEmail := &parser.MailReceived{
		NotificationType: notificationPayload.Type,
	}

	return mimeEmail, nil
}
