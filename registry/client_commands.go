/***************************************************************
 *
 * Copyright (C) 2024, Pelican Project, Morgridge Institute for Research
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you
 * may not use this file except in compliance with the License.  You may
 * obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 ***************************************************************/

package registry

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/pelicanplatform/pelican/config"
	"github.com/pelicanplatform/pelican/server_utils"
	"github.com/pelicanplatform/pelican/token"
	"github.com/pelicanplatform/pelican/token_scopes"
	"github.com/pelicanplatform/pelican/utils"
)

type clientResponseData struct {
	VerificationURL string            `json:"verification_url"`
	DeviceCode      string            `json:"device_code"`
	Status          string            `json:"status"`
	AccessToken     string            `json:"access_token"`
	ServerNonce     string            `json:"server_nonce"`
	ServerPayload   string            `json:"server_payload"`
	ServerSignature string            `json:"server_signature"`
	RegisteredKeys  map[string]string `json:"registered_keys"`
	Message         string            `json:"msg"`
	Error           string            `json:"error"`
}

func NamespaceRegisterWithIdentity(privateKey jwk.Key, namespaceRegistryEndpoint string, prefix string, siteName string) error {
	identifiedPayload := map[string]interface{}{
		"identity_required": "true",
		"prefix":            prefix,
		// we'll also send the prefix so we can avoid more work if
		// it's also registered already

	}
	tr := config.GetTransport()
	resp, err := utils.MakeRequest(context.Background(), tr, namespaceRegistryEndpoint, "POST", identifiedPayload, nil)

	var respData clientResponseData
	// Handle case where there was an error encoded in the body
	if err != nil {
		if unmarshalErr := json.Unmarshal(resp, &respData); unmarshalErr == nil { // Error creating json
			return errors.Wrapf(err, "Failed to make request: %v", respData.Error)
		}
		return errors.Wrap(err, "Failed to make request")
	}

	// no error
	if err = json.Unmarshal(resp, &respData); err != nil {
		return errors.Wrap(err, "Failure when parsing JSON response from client")
	}
	fmt.Printf("Verification URL:\n%s\n", respData.VerificationURL)

	done := false
	for !done {
		identifiedPayload = map[string]interface{}{
			"identity_required": "true",
			"device_code":       respData.DeviceCode,
		}
		resp, err = utils.MakeRequest(context.Background(), tr, namespaceRegistryEndpoint, "POST", identifiedPayload, nil)
		if err != nil {
			return errors.Wrap(err, "Failed to make request")
		}

		if err = json.Unmarshal(resp, &respData); err != nil {
			return errors.Wrap(err, "Failure when parsing JSON response from client")
		}

		if respData.Status == "APPROVED" {
			done = true
		} else {
			fmt.Println("Waiting for approval. Press Enter after verification.")
			reader := bufio.NewReader(os.Stdin)
			_, _ = reader.ReadString('\n')
		}
	}
	return NamespaceRegister(privateKey, namespaceRegistryEndpoint, respData.AccessToken, prefix, siteName)
}

func NamespaceRegister(privateKey jwk.Key, namespaceRegistryEndpoint string, accessToken string, prefix string, siteName string) error {
	publicKey, err := privateKey.PublicKey()
	if err != nil {
		return errors.Wrapf(err, "failed to generate public key for namespace registration")
	}
	err = jwk.AssignKeyID(publicKey)
	if err != nil {
		return errors.Wrap(err, "failed to assign key ID to public key")
	}
	if err = publicKey.Set("alg", "ES256"); err != nil {
		return errors.Wrap(err, "failed to assign signature algorithm to public key")
	}
	keySet := jwk.NewSet()
	if err = keySet.AddKey(publicKey); err != nil {
		return errors.Wrap(err, "failed to add public key to new JWKS")
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		// Let's check that we can convert to JSON and get the right thing...
		jsonbuf, err := json.Marshal(keySet)
		if err != nil {
			return errors.Wrap(err, "failed to marshal the public key into JWKS JSON")
		}
		log.Debugln("Constructed JWKS from loading public key:", string(jsonbuf))
	}

	clientNonce, err := generateNonce()
	if err != nil {
		return errors.Wrap(err, "failed to generate client nonce")
	}

	data := map[string]interface{}{
		"client_nonce": clientNonce,
		"pubkey":       keySet,
	}

	tr := config.GetTransport()
	resp, err := utils.MakeRequest(context.Background(), tr, namespaceRegistryEndpoint, "POST", data, nil)

	var respData clientResponseData
	// Handle case where there was an error encoded in the body
	if err != nil {
		if unmarshalErr := json.Unmarshal(resp, &respData); unmarshalErr == nil {
			return errors.Wrapf(err, "Server responded with an error: %s. %s", respData.Message, respData.Error)
		}
		return errors.Wrapf(err, "Server responded with an error and failed to parse JSON response from the server. Raw server response is %s", resp)
	}

	// No error
	if err = json.Unmarshal(resp, &respData); err != nil {
		return errors.Wrapf(err, "Failure when parsing JSON response from the server with a success request. Raw server response is %s", resp)
	}

	// Create client payload by concatenating client_nonce and server_nonce
	clientPayload := clientNonce + respData.ServerNonce

	// Sign the payload
	privateKeyRaw := &ecdsa.PrivateKey{}
	if err = privateKey.Raw(privateKeyRaw); err != nil {
		return errors.Wrap(err, "failed to get an ECDSA private key")
	}
	signature, err := signPayload([]byte(clientPayload), privateKeyRaw)
	if err != nil {
		return errors.Wrap(err, "failed to sign payload")
	}

	// // Create data for the second POST request
	unidentifiedPayload := map[string]interface{}{
		"pubkey":            keySet,
		"prefix":            prefix,
		"site_name":         siteName,
		"client_nonce":      clientNonce,
		"server_nonce":      respData.ServerNonce,
		"client_payload":    clientPayload,
		"client_signature":  hex.EncodeToString(signature),
		"server_payload":    respData.ServerPayload,
		"server_signature":  respData.ServerSignature,
		"access_token":      accessToken,
		"identity_required": "false",
	}

	// Send the second POST request
	resp, err = utils.MakeRequest(context.Background(), tr, namespaceRegistryEndpoint, "POST", unidentifiedPayload, nil)

	// Handle case where there was an error encoded in the body
	if unmarshalErr := json.Unmarshal(resp, &respData); unmarshalErr == nil {
		if err != nil {
			log.Errorf("Server responded with an error: %v. %s. %s", respData.Message, respData.Error, err)
			return errors.Wrapf(err, "Server responded with an error: %s. %s", respData.Message, respData.Error)
		}
		if respData.Message != "" {
			log.Debugf("Server responded to registration confirmation successfully with message: %s", respData.Message)
		}
	} else { // Error decoding JSON
		if err != nil {
			return errors.Wrapf(err, "Server responded with an error and failed to parse JSON response from the server. Raw response is %s", resp)
		}
		return errors.Wrapf(unmarshalErr, "Failure when parsing JSON response from the server with a success request. Raw server response is %s", resp)
	}

	return nil
}

func NamespaceList(endpoint string) error {
	tr := config.GetTransport()
	respData, err := utils.MakeRequest(context.Background(), tr, endpoint, "GET", nil, nil)
	var respErr clientResponseData
	if err != nil {
		if jsonErr := json.Unmarshal(respData, &respErr); jsonErr == nil { // Error creating json
			return errors.Wrapf(err, "Failed to make request: %v", respErr.Error)
		}
		return errors.Wrap(err, "Failed to make request")
	}
	fmt.Println(string(respData))
	return nil
}

func NamespaceGet(endpoint string) error {
	tr := config.GetTransport()
	respData, err := utils.MakeRequest(context.Background(), tr, endpoint, "GET", nil, nil)
	var respErr clientResponseData
	if err != nil {
		if jsonErr := json.Unmarshal(respData, &respErr); jsonErr == nil { // Error creating json
			return errors.Wrapf(err, "Failed to make request: %v", respErr.Error)
		}
		return errors.Wrap(err, "Failed to make request")
	}
	fmt.Println(string(respData))
	return nil
}

func NamespaceDelete(endpoint string, prefix string) error {
	// First we create a token for the registry to check that the deletion
	// request is valid

	issuerURL, err := server_utils.GetNSIssuerURL(prefix)
	if err != nil {
		return errors.Wrap(err, "Failed to determine prefix's issuer/pubkey URL for creating deletion token")
	}

	// TODO: Eventually we should think about a naming scheme for
	//       including an audience with these tokens.
	// TODO: Investigate whether 1 min is a good expiration interval
	//       or whether this should be altered.
	delTokenCfg := token.NewWLCGToken()
	delTokenCfg.Lifetime = time.Minute
	delTokenCfg.Issuer = issuerURL
	delTokenCfg.AddAudiences("registry")
	delTokenCfg.Subject = "origin"
	delTokenCfg.AddScopes(token_scopes.Pelican_NamespaceDelete)

	// CreateToken also handles validation for us
	tok, err := delTokenCfg.CreateToken()
	if err != nil {
		return errors.Wrap(err, "failed to create namespace deletion token")
	}

	// We're at the client, so it *should* be safe to print the signed token to
	// stdout when the client asks for debug -- a future attacker will only find
	// expired tokens, and an attacker with current access can just use the priv
	// key to create their own. Famous last words?
	log.Debugln("Signed deletion token:", tok)

	authHeader := map[string]string{
		"Authorization": "Bearer " + tok,
	}
	tr := config.GetTransport()
	respData, err := utils.MakeRequest(context.Background(), tr, endpoint, "DELETE", nil, authHeader)
	var respErr clientResponseData
	if err != nil {
		if unmarshalErr := json.Unmarshal(respData, &respErr); unmarshalErr == nil { // Error creating json
			return errors.Wrapf(err, "Failed to make request: %v", respErr.Error)
		}
		return errors.Wrap(err, "Failed to make request")
	}
	fmt.Println(string(respData))
	return nil
}

// Update the set of registered public keys for a given list of prefixes & site name.
//
//   - `privateKey`: The private key to use to sign the request (must also be one of the
//     private keys in the configuration).
//   - `prefixes`: The list of prefixes to update the public keys for in the registry.
//   - `siteName`: The name that the server is registered under.
//   - `namespacePubKeyEndUpdatepoint`: The registry endpoint.
func NamespacesPubKeyUpdate(privateKey jwk.Key, prefixes []string, siteName string, namespacePubKeyEndUpdatepoint string) error {
	publicKey, err := privateKey.PublicKey()
	if err != nil {
		return errors.Wrapf(err, "failed to generate public key for namespace registration")
	}
	if err = jwk.AssignKeyID(publicKey); err != nil {
		return errors.Wrap(err, "failed to assign key ID to public key")
	}
	if err = publicKey.Set("alg", "ES256"); err != nil {
		return errors.Wrap(err, "failed to assign signature algorithm to public key")
	}
	keySet := jwk.NewSet()
	if err = keySet.AddKey(publicKey); err != nil {
		return errors.Wrap(err, "failed to add public key to new JWKS")
	}

	// Let's check that we can convert to JSON and get the right thing...
	jsonbuf, err := json.Marshal(keySet)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the public key into JWKS JSON")
	}
	log.Debugln("Constructed JWKS from loading public key:", string(jsonbuf))

	clientNonce, err := generateNonce()
	if err != nil {
		return errors.Wrap(err, "failed to generate client nonce")
	}

	data := map[string]interface{}{
		"client_nonce": clientNonce,
		"pubkey":       keySet,
		"prefixes":     prefixes,
	}

	tr := config.GetTransport()
	resp, err := utils.MakeRequest(context.Background(), tr, namespacePubKeyEndUpdatepoint, "POST", data, nil)

	var respData clientResponseData
	// Handle case where there was an error encoded in the body
	if err != nil {
		if strings.Contains(err.Error(), "status code 404") {
			log.Warnf("Registered namespace public key  enUpdatedpoint returned 404 Not Found: %s. This endpoint is available in Pelican v7.12 or later.", namespacePubKeyEndUpdatepoint)
			return nil
		}
		if unmarshalErr := json.Unmarshal(resp, &respData); unmarshalErr == nil {
			return errors.Wrapf(err, "Server responded with an error: %s. %s", respData.Message, respData.Error)
		}
		return errors.Wrapf(err, "Server responded with an error and failed to parse JSON response from the server. Raw server response is %s", resp)
	}

	// No error
	if err = json.Unmarshal(resp, &respData); err != nil {
		return errors.Wrapf(err, "Failure when parsing JSON response from the server with a successful request. Raw server response is %s", resp)
	}

	// Create client payload by concatenating client_nonce and server_nonce
	clientPayload := clientNonce + respData.ServerNonce

	// Sign the payload
	privateKeyRaw := &ecdsa.PrivateKey{}
	if err = privateKey.Raw(privateKeyRaw); err != nil {
		return errors.Wrap(err, "failed to get an ECDSA private key")
	}
	signature, err := signPayload([]byte(clientPayload), privateKeyRaw)
	if err != nil {
		return errors.Wrap(err, "failed to sign payload")
	}

	privateKeys := config.GetIssuerPrivateKeys()
	if len(privateKeys) == 0 {
		return errors.Errorf("The server doesn't have any private key in memory")
	}
	issuerPubKeysSet := jwk.NewSet()
	for _, privKey := range privateKeys {
		pubKey, err := privKey.PublicKey()
		if err != nil {
			return errors.Wrapf(err, "failed to get the public key of a private key")
		}
		if err = issuerPubKeysSet.AddKey(pubKey); err != nil {
			return errors.Wrap(err, "failed to add public key to all keys JWKS")
		}
	}

	var matchedKeyId string
	var keyUpdateAuthzSignature []byte
	for _, prefix := range prefixes {
		// Parse the public key(s) of the registered prefix
		registeredKeys := respData.RegisteredKeys[prefix]
		registryDbKeySet, err := jwk.ParseString(registeredKeys)
		if err != nil {
			log.Errorf("Failed to parse public key as JWKS of registered namespace %s: %v", prefix, err)
			return errors.Wrapf(err, "Invalid public key format of registered namespace %s", prefix)
		}
		for i := 0; i < registryDbKeySet.Len(); i++ {
			registeredPubKey, ok := registryDbKeySet.Key(i)
			if !ok {
				continue
			}
			// Look for the issuer public key this origin previously registered the namespace with
			issuerPubKey, found := issuerPubKeysSet.LookupKeyID(registeredPubKey.KeyID())
			if !found {
				continue
			}
			// Sign the payload with the issuer private key matched with the registered public key
			issuerPrivKey := privateKeys[issuerPubKey.KeyID()]
			rawkey := &ecdsa.PrivateKey{}
			if err := issuerPrivKey.Raw(rawkey); err != nil {
				return errors.Wrap(err, "failed to generate raw private key from the issuer private key matched with the registered public key")
			}
			if keyUpdateAuthzSignature, err = signPayload([]byte(clientPayload), rawkey); err != nil {
				return errors.Wrap(err, "failed to sign the payload with the issuer private key matched with the registered public key")
			}
			matchedKeyId = issuerPubKey.KeyID()
			break
		}
	}
	if keyUpdateAuthzSignature == nil || matchedKeyId == "" {
		return errors.Errorf("Origin does not possess valid key to update public keys of registered namespace(s)")
	}

	// Create data for the second POST request
	unidentifiedPayload := map[string]interface{}{
		"pubkey":                     keySet,
		"all_pubkeys":                issuerPubKeysSet,
		"prefixes":                   prefixes,
		"site_name":                  siteName,
		"client_nonce":               clientNonce,
		"server_nonce":               respData.ServerNonce,
		"client_payload":             clientPayload,
		"client_signature":           hex.EncodeToString(signature),
		"key_update_authz_signature": hex.EncodeToString(keyUpdateAuthzSignature),
		"matched_key_id":             matchedKeyId,
		"server_payload":             respData.ServerPayload,
		"server_signature":           respData.ServerSignature,
	}

	// Send the second POST request
	resp, err = utils.MakeRequest(context.Background(), tr, namespacePubKeyEndUpdatepoint, "POST", unidentifiedPayload, nil)

	// Handle case where there was an error encoded in the body
	if unmarshalErr := json.Unmarshal(resp, &respData); unmarshalErr == nil {
		if err != nil {
			if strings.Contains(err.Error(), "status code 404") {
				log.Warnf("Registered namespace public key  enUpdatedpoint returned 404 Not Found: %s. This endpoint is available in Pelican v7.12 or later.", namespacePubKeyEndUpdatepoint)
				return nil
			}
			log.Errorf("Server responded with an error: %v. %s. %s", respData.Message, respData.Error, err)
			return errors.Wrapf(err, "Server responded with an error: %s. %s", respData.Message, respData.Error)
		}
		if respData.Message != "" {
			log.Debugf("Server responded to registration confirmation successfully with message: %s", respData.Message)
		}
	} else { // Error decoding JSON
		if err != nil {
			return errors.Wrapf(err, "Server responded with an error and failed to parse JSON response from the server. Raw response is %s", resp)
		}
		return errors.Wrapf(unmarshalErr, "Failure when parsing JSON response from the server with a successful request. Raw server response is %s", resp)
	}

	return nil
}
