package gssapi

import (
	"time"

	"github.com/jcmturner/gofork/encoding/asn1"
	"github.com/jcmturner/gokrb5/v8/asn1tools"
	"github.com/jcmturner/gokrb5/v8/crypto"
	"github.com/jcmturner/gokrb5/v8/iana"
	"github.com/jcmturner/gokrb5/v8/iana/asnAppTag"
	"github.com/jcmturner/gokrb5/v8/iana/keyusage"
	"github.com/jcmturner/gokrb5/v8/iana/msgtype"
	"github.com/jcmturner/gokrb5/v8/krberror"
	"github.com/jcmturner/gokrb5/v8/messages"
	"github.com/jcmturner/gokrb5/v8/types"
)

// These are a 1:1 copy of the types from github.com/jcmturner/gokrb5/v8
// with marshalling methods added. If/when upstream adds the missing methods
// these can be removed.

type apRep struct {
	PVNO    int                 `asn1:"explicit,tag:0"`
	MsgType int                 `asn1:"explicit,tag:1"`
	EncPart types.EncryptedData `asn1:"explicit,tag:2"`
}

func (a *apRep) marshal() (b []byte, err error) {
	b, err = asn1.Marshal(*a)
	if err != nil {
		return
	}

	b = asn1tools.AddASNAppTag(b, asnAppTag.APREP)

	return
}

type encAPRepPart struct {
	CTime          time.Time           `asn1:"generalized,explicit,tag:0"`
	Cusec          int                 `asn1:"explicit,tag:1"`
	Subkey         types.EncryptionKey `asn1:"optional,explicit,tag:2"`
	SequenceNumber int64               `asn1:"optional,explicit,tag:3"`
}

func (a *encAPRepPart) marshal() (b []byte, err error) {
	b, err = asn1.Marshal(*a)
	if err != nil {
		return
	}

	b = asn1tools.AddASNAppTag(b, asnAppTag.EncAPRepPart)

	return
}

func newAPRep(tkt messages.Ticket, sessionKey types.EncryptionKey, encPart encAPRepPart) (a apRep, err error) {
	m, err := encPart.marshal()
	if err != nil {
		err = krberror.Errorf(err, krberror.EncodingError, "marshaling error of AP-REP enc-part")

		return
	}

	ed, err := crypto.GetEncryptedData(m, sessionKey, keyusage.AP_REP_ENCPART, tkt.EncPart.KVNO)
	if err != nil {
		err = krberror.Errorf(err, krberror.EncryptingError, "error encrypting AP-REP enc-part")

		return
	}

	a = apRep{
		PVNO:    iana.PVNO,
		MsgType: msgtype.KRB_AP_REP,
		EncPart: ed,
	}

	return
}
