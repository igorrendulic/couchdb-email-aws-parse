package awshandler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
)

func LoadPayload(filename string) ([]byte, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	return byteValue, nil
}

func TestAwsHandlerReceivedMsgParse(t *testing.T) {
	payload, err := LoadPayload("test_data/received.json")
	if err != nil {
		t.Fatal(err)
	}
	// should map to MessageJSON
	var msg MessageJSON
	err = json.Unmarshal(payload, &msg)
	if err != nil {
		t.Fatalf("received.json expected to map to MessageJSONerr: %v\n", err)
	}
}

func TestAwsHandlerReceivedMsgHandle(t *testing.T) {
	payload, err := LoadPayload("test_data/received.json")
	if err != nil {
		t.Fatal(err)
	}
	awsSess := &session.Session{}
	smtpHandler := NewAwsSmtpHandler(awsSess)
	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err == SignatureInvalid {
		t.Fatalf("received.json failed signature validation %v\n", err)
	}
	if err != nil {
		t.Fatalf("received.json expected to be handled without error: %v\n", err)
	}
	fmt.Printf("mailReceived: %v\n", mailReceived)
}

func TestSubscriptionConfirmation(t *testing.T) {
	payload, err := LoadPayload("test_data/subscription-confirmation.json")
	if err != nil {
		t.Fatal(err)
	}
	awsSess := &session.Session{}
	smtpHandler := NewAwsSmtpHandler(awsSess)
	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err != nil {
		t.Fatalf("subscription-confirmation.json expected to be handled without error: %v\n", err)
	}
	if mailReceived.NotificationType != "SubscriptionConfirmation" {
		t.Fatalf("NotificationType expected to be SubscriptionConfirmation: %v\n", err)
	}
}
