package token

import (
	"bytes"
	"encoding/gob"
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauth "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	"k8s.io/utils/ptr"

	"github.com/seal-io/kubecia/pkg/json"
)

func init() {
	gob.Register(_Token{})
}

type (
	Token struct {
		Expiration time.Time `json:"expiration,omitempty"`
		Value      string    `json:"value"`
	}

	// _Token alias Token, see https://github.com/golang/go/issues/32251.
	_Token Token
)

func (t *Token) Expired() bool {
	if t.Expiration.IsZero() {
		return true
	}

	return t.Expiration.Before(time.Now())
}

func (t *Token) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	err := gob.NewEncoder(&buf).Encode(_Token(*t))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t *Token) UnmarshalBinary(b []byte) error {
	var _t _Token

	err := gob.NewDecoder(bytes.NewReader(b)).Decode(&_t)
	if err != nil {
		return err
	}

	*t = Token(_t)

	return nil
}

func (t *Token) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(*t)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t *Token) UnmarshalJSON(b []byte) error {
	return json.NewDecoder(bytes.NewReader(b)).Decode(t)
}

func (t *Token) ToKubeClientExecCredential() clientauth.ExecCredential {
	return clientauth.ExecCredential{
		TypeMeta: meta.TypeMeta{
			APIVersion: clientauth.SchemeGroupVersion.String(),
			Kind:       "ExecCredential",
		},
		Status: &clientauth.ExecCredentialStatus{
			ExpirationTimestamp: ptr.To(meta.NewTime(t.Expiration)),
			Token:               t.Value,
		},
	}
}

func (t *Token) ToKubeClientExecCredentialJSON() ([]byte, error) {
	return json.Marshal(t.ToKubeClientExecCredential())
}
