package awsauth

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	. "github.com/smartystreets/goconvey/convey"
)

func TestVersion4RequestPreparer(t *testing.T) {
	Convey("Given a plain request with no custom headers", t, func() {
		req := test_plainRequestV4(false)

		expectedUnsigned := test_unsignedRequestV4(true)
		expectedUnsigned.Header.Set("X-Amz-Date", timestampV4())

		Convey("The necessary, default headers should be appended", func() {
			prepareRequestV4(req)
			So(req, ShouldResemble, expectedUnsigned)
		})

		Convey("Forward-slash should be appended to URI if not present", func() {
			prepareRequestV4(req)
			So(req.URL.Path, ShouldEqual, "/")
		})

		Convey("And a set of credentials", func() {
			Keys = testCredV4

			Convey("It should be signed with an Authorization header", func() {
				actualSigned := Sign4(req)
				actual := actualSigned.Header.Get("Authorization")

				So(actual, ShouldNotBeBlank)
				So(actual, ShouldContainSubstring, "Credential="+testCredV4.AccessKeyID)
				So(actual, ShouldContainSubstring, "SignedHeaders=")
				So(actual, ShouldContainSubstring, "Signature=")
				So(actual, ShouldContainSubstring, "AWS4")
			})
		})
	})

	Convey("Given a request with custom, necessary headers", t, func() {
		Convey("The custom, necessary headers must not be changed", func() {
			req := test_unsignedRequestV4(true)
			prepareRequestV4(req)
			So(req, ShouldResemble, test_unsignedRequestV4(true))
		})
	})
}

func TestVersion4STSRequestPreparer(t *testing.T) {
	Convey("Given a plain request with no custom headers", t, func() {
		req := test_plainRequestV4(false)

		Convey("And a set of credentials with an STS token", func() {
			Keys = testCredV4WithSTS

			Convey("It should include an X-Amz-Security-Token when the request is signed", func() {
				actualSigned := Sign4(req)
				actual := actualSigned.Header.Get("X-Amz-Security-Token")

				So(actual, ShouldNotBeBlank)
				So(actual, ShouldEqual, testCredV4WithSTS.SecurityToken)

			})
		})
	})

}

func TestVersion4SigningTasks(t *testing.T) {
	// http://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html

	Convey("Given a bogus request and credentials from AWS documentation", t, func() {
		req := test_unsignedRequestV4(true)
		meta := new(metadata)

		Convey("(Task 1) The canonical request should be built correctly", func() {
			hashedCanonReq := hashedCanonicalRequestV4(req, meta)

			So(hashedCanonReq, ShouldEqual, expectingV4["CanonicalHash"])
		})

		Convey("(Task 2) The string to sign should be built correctly", func() {
			hashedCanonReq := hashedCanonicalRequestV4(req, meta)
			stringToSign := stringToSignV4(req, hashedCanonReq, meta)

			So(stringToSign, ShouldEqual, expectingV4["StringToSign"])
		})

		Convey("(Task 3) The version 4 signed signature should be correct", func() {
			hashedCanonReq := hashedCanonicalRequestV4(req, meta)
			stringToSign := stringToSignV4(req, hashedCanonReq, meta)
			signature := signatureV4(test_signingKeyV4(), stringToSign)

			So(signature, ShouldEqual, expectingV4["SignatureV4"])
		})
	})
}

func TestSignature4Helpers(t *testing.T) {
	Convey("The signing key should be properly generated", t, func() {
		expected := []byte{152, 241, 216, 137, 254, 196, 244, 66, 26, 220, 82, 43, 171, 12, 225, 248, 46, 105, 41, 194, 98, 237, 21, 229, 169, 76, 144, 239, 209, 227, 176, 231}
		actual := test_signingKeyV4()

		So(actual, ShouldResemble, expected)
	})

	Convey("Authorization headers should be built properly", t, func() {
		meta := &metadata{
			algorithm:       "AWS4-HMAC-SHA256",
			credentialScope: "20110909/us-east-1/iam/aws4_request",
			signedHeaders:   "content-type;host;x-amz-date",
		}
		expected := expectingV4["AuthHeader"] + expectingV4["SignatureV4"]
		actual := buildAuthHeaderV4(expectingV4["SignatureV4"], meta)

		So(actual, ShouldEqual, expected)
	})

	Convey("Timestamps should be in the correct format, in UTC time", t, func() {
		actual := timestampV4()

		So(len(actual), ShouldEqual, 16)
		So(actual, ShouldNotContainSubstring, ":")
		So(actual, ShouldNotContainSubstring, "-")
		So(actual, ShouldNotContainSubstring, " ")
		So(actual, ShouldEndWith, "Z")
		So(actual, ShouldContainSubstring, "T")
	})

	Convey("Given an Version 4 AWS-formatted timestamp", t, func() {
		ts := "20110909T233600Z"

		Convey("The date string should be extracted properly", func() {
			So(tsDateV4(ts), ShouldEqual, "20110909")
		})
	})

	Convey("Given any request with a body", t, func() {
		req := test_plainRequestV4(false)

		Convey("Its body should be read and replaced without differences", func() {
			expected := []byte(requestValuesV4.Encode())

			actual1 := readAndReplaceBody(req)
			So(actual1, ShouldResemble, expected)

			actual2 := readAndReplaceBody(req)
			So(actual2, ShouldResemble, expected)
		})
	})
}

func test_plainRequestV4(trailingSlash bool) *http.Request {
	url := "http://iam.amazonaws.com"
	body := strings.NewReader(requestValuesV4.Encode())

	if trailingSlash {
		url += "/"
	}

	req, err := http.NewRequest("POST", url, body)

	if err != nil {
		panic(err)
	}

	return req
}

func test_unsignedRequestV4(trailingSlash bool) *http.Request {
	req := test_plainRequestV4(trailingSlash)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("X-Amz-Date", "20110909T233600Z")
	return req
}

func test_signingKeyV4() []byte {
	return signingKeyV4(testCredV4.SecretAccessKey, "20110909", "us-east-1", "iam")
}

var (
	testCredV4 = &Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	}

	testCredV4WithSTS = &Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		SecurityToken:   "AQoDYXdzEHcaoAJ1Aqwx1Sum0iW2NQjXJcWlKR7vuB6lnAeGBaQnjDRZPVyniwc48ml5hx+0qiXenVJdfusMMl9XLhSncfhx9Rb1UF8IAOaQ+CkpWXvoH67YYN+93dgckSVgVEBRByTl/BvLOZhe0ii/pOWkuQtBm5T7lBHRe4Dfmxy9X6hd8L3FrWxgnGV3fWZ3j0gASdYXaa+VBJlU0E2/GmCzn3T+t2mjYaeoInAnYVKVpmVMOrh6lNAeETTOHElLopblSa7TAmROq5xHIyu4a9i2qwjERTwa3Yk4Jk6q7JYVA5Cu7kS8wKVml8LdzzCTsy+elJgvH+Jf6ivpaHt/En0AJ5PZUJDev2+Y5+9j4AYfrmXfm4L73DC1ZJFJrv+Yh+EXAMPLE=",
	}

	expectingV4 = map[string]string{
		"CanonicalHash": "6bb0c1d1a458667c2717e3b2f7b14033f757a8e7230013d40b1e4d18b2378fe4",
		"StringToSign":  "AWS4-HMAC-SHA256\n20110909T233600Z\n20110909/us-east-1/iam/aws4_request\n6bb0c1d1a458667c2717e3b2f7b14033f757a8e7230013d40b1e4d18b2378fe4",
		"SignatureV4":   "2f5b7bfe9c47bbf5fff60c6d667e1439f64a29f3f194fa02d83facf42843fa8d",
		"AuthHeader":    "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/iam/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=",
	}

	requestValuesV4 = &url.Values{
		"Action":  []string{"ListUsers"},
		"Version": []string{"2010-05-08"},
	}
)
