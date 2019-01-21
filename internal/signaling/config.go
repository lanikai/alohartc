package signaling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type KeyPair struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// Config contains credentials for a Signal Service Device.
type Config struct {
	// The ARN of the certificate.
	CertificateArn string `json:"certificateArn"`

	// The ID of the certificate. AWS IoT issues a default subject name for the
	// certificate (e.g., AWS IoT Certificate).
	CertificateID string `json:"certificateID"`

	// Service endpoint issued by IOT
	ServiceEndpoint string `json:"serviceEndpoint"`

	// DeviceId (like SN)
	DeviceId string `json:"deviceId"`

	// The owner of this device
	Owner string `json:"owner"`

	// Account + stage ID combined
	AccountStageId string `json:"accountStageId"`

	// The certificate data, in PEM format.
	CertificatePem string `json:"certificatePem"`

	// The stage that the Signaling Service is using
	Stage string `json:"stage"`

	// The generated key pair.
	KeyPair *KeyPair `json:"keyPair"`

	// Debug flag
	Debug bool `json:"debug"`
}

// LoadConfig load the configuration from a file
func LoadConfig(filePath string) (*Config, error) {

	tc := &Config{}

	d, err := ioutil.ReadFile(filePath)

	if err != nil {
		return tc, err
	}

	return tc, json.Unmarshal(d, &tc)
}

/**
 * The owner is replaced with a single dollar sign if it's empty
 * since $ is a reserved character inside the signaling service.
 * (This is the agreed upon value for "Empty Owner")
 *
 * @return {String}             A safe to use value for owner in topics
 */
func (config *Config) getOwnerNullSafe() string {
	ownerNullSafe := config.Owner
	if ownerNullSafe == "" {
		ownerNullSafe = "$"
	}
	return ownerNullSafe
}

/**
 * This is a topic which is listened to by a given client - we are
 * not allowed to listen to anything more permissive or else we will
 * get a permission error
 *
 * @return {String}             Topic string to start listening with.
 */
func (config *Config) getSignalingReceivingTopic() string {
	return fmt.Sprintf("%s-sC/%s/%s/%s/+/+/",
		config.Stage,
		config.AccountStageId,
		config.getOwnerNullSafe(),
		config.DeviceId)
}

/**
 * This is a topic which is listened to by every client which wishes
 * to connect to this device. It is used for updating status for this
 * device that everyone needs to be alerted to.
 *
 * @return {String}             Topic string to broadcast with
 */
func (config *Config) getBroadcastTopic() string {
	return fmt.Sprintf("%s-bH/%s/%s/%s/",
		config.Stage,
		config.AccountStageId,
		config.getOwnerNullSafe(),
		config.DeviceId)
}

/**
 * This is a topic which is listened to by a client specifically
 * to _receive_ messages from only the server
 *
 * @return {String}             Topic string to start listening with.
 */
func (config *Config) getServerReceivingTopic() string {
	return fmt.Sprintf("%s-rS/%s/%s/%s/",
		config.Stage,
		config.AccountStageId,
		config.getOwnerNullSafe(),
		config.DeviceId)
}

/**
 * This is a topic which is listened to by the server such that
 * the device can interact with the server using the lowest latency
 * mechanism available.
 *
 * @return {String}             Topic string to send with
 */
func (config *Config) getMessageToServerTopic() string {
	return fmt.Sprintf("%s-mS/%s/%s/%s/",
		config.Stage,
		config.AccountStageId,
		config.getOwnerNullSafe(),
		config.DeviceId)
}
