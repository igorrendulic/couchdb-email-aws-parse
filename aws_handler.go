package awshandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/igorrendulic/couchdb-email-aws-parse/sns"
	"github.com/igorrendulic/couchdb-experiment/email/mime/handler"
)

// errors
var S3FileNotFoundError = errors.New("s3 mime content file not found")
var SignatureInvalid = errors.New("could not verify signature")

type AwsSmtpHandler struct {
	awsSession *session.Session
}

func NewAwsSmtpHandler(awsSession *session.Session) handler.SmtpHandler {
	return &AwsSmtpHandler{
		awsSession: awsSession,
	}
}

// Handle SMTP SNS topic notification received by AWS SES (Simple Email Service)
func (p *AwsSmtpHandler) HandleSmtp(message []byte) (*handler.MailReceived, error) {

	var notificationPayload sns.Payload
	err := json.Unmarshal(message, &notificationPayload)
	if err != nil {
		return nil, err
	}

	indentJson, _ := json.MarshalIndent(notificationPayload, "", "  ")
	fmt.Printf("notificationPayload: %s", indentJson)

	//TODO! check if bounce or any other type of message (check key maybe?)

	verifyErr := notificationPayload.VerifyPayload()
	if verifyErr != nil {
		return nil, SignatureInvalid
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
		return &handler.MailReceived{
			NotificationType: notificationPayload.Type,
			Timestamp:        ts.UnixMilli(),
		}, nil
	}
	if notificationPayload.Type == "Notification" {
		message := notificationPayload.Message
		var messageJSON MessageJSON
		err := json.Unmarshal([]byte(message), &messageJSON)
		if err != nil {
			return nil, err
		}
		if messageJSON.NotificationType == "Received" {
			mail := messageJSON.Mail
			receipt := messageJSON.Receipt

			output := &handler.MailReceived{}

			bucket, key, bkErr := p.extractS3PathToContent(receipt)
			if bkErr != nil {
				return nil, bkErr
			}
			mimeBytes, mErr := p.downloadS3File(bucket, key)
			if mErr != nil {
				return nil, mErr
			}

			output.NotificationType = "unknown just yet (complaint or bounce or delivery or receipt)"
			output.Timestamp = ts.UnixMilli()

			output.Mail = &handler.Mail{
				RawMime:     mimeBytes,
				Source:      mail.Source,
				Destination: mail.Destination,
				MessageID:   mail.MessageID,
			}

			if mail.CommonHeaders != nil {
				output.Mail.CommonHeaders = &handler.CommonHeaders{
					From:      mail.CommonHeaders.From,
					Subject:   mail.CommonHeaders.Subject,
					To:        mail.CommonHeaders.To,
					MessageID: mail.CommonHeaders.MessageID,
				}
			}

			s3Url := "s3://" + receipt.Action.BucketName
			if receipt.Action.ObjectKeyPrefix != "" {
				s3Url += "/" + receipt.Action.ObjectKeyPrefix
			}
			s3Url += "/" + receipt.Action.ObjectKey

			output.Receipt = &handler.Receipt{
				Action: &handler.Action{
					Type:      receipt.Action.Type,
					Topic:     receipt.Action.TopicArn,
					ObjectURL: s3Url,
				},
				Recipients:           receipt.Recipients,
				ProcessingTimeMillis: receipt.ProcessingTimeMillis,
				SpamVerdict: &handler.VerdictStatus{
					Status: receipt.SpamVerdict.Status,
				},
				VirusVerdict: &handler.VerdictStatus{
					Status: receipt.VirusVerdict.Status,
				},
				SpfVerdict: &handler.VerdictStatus{
					Status: receipt.SpfVerdict.Status,
				},
				DkimVerdict: &handler.VerdictStatus{
					Status: receipt.DkimVerdict.Status,
				},
			}

			return output, nil
		}
	}

	mimeEmail := &handler.MailReceived{
		NotificationType: notificationPayload.Type,
	}

	return mimeEmail, nil
}

func (p *AwsSmtpHandler) GetHandlerName() string {
	return "aws"
}

func (p *AwsSmtpHandler) extractS3PathToContent(receipt *Receipt) (string, string, error) {
	bucket := ""
	key := ""
	if receipt != nil {
		action := receipt.Action
		if action != nil {
			bucket = action.BucketName
			key = action.ObjectKey
			if action.ObjectKeyPrefix != "" {
				key = action.ObjectKeyPrefix + "/" + action.ObjectKey
			}
		}
	}
	if bucket == "" || key == "" {
		return "", "", S3FileNotFoundError
	}
	return bucket, key, nil
}

func (p *AwsSmtpHandler) downloadS3File(bucket string, key string) ([]byte, error) {
	downloader := s3manager.NewDownloader(p.awsSession)

	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
