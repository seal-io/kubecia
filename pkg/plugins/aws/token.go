package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/cache"
	"github.com/seal-io/kubecia/pkg/token"
)

type TokenOptions struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Cluster         string
	AssumeRoleARN   string
}

func (o *TokenOptions) Validate() error {
	var requiredTenant bool

	if strings.HasPrefix(o.AccessKeyID, "$") {
		o.AccessKeyID = os.ExpandEnv(o.AccessKeyID)
		requiredTenant = true
	}

	if o.AccessKeyID == "" {
		if requiredTenant {
			return errors.New("hosted access key ID is required")
		}

		return errors.New("access key ID is required")
	}

	if strings.HasPrefix(o.SecretAccessKey, "$") {
		o.SecretAccessKey = os.ExpandEnv(o.SecretAccessKey)
		requiredTenant = true
	}

	if o.SecretAccessKey == "" {
		if requiredTenant {
			return errors.New("hosted secret access key is required")
		}

		return errors.New("secret access key is required")
	}

	if o.Region == "" {
		return errors.New("region is required")
	}

	if o.Cluster == "" {
		return errors.New("cluster ID is required")
	}

	if o.AssumeRoleARN == "" && requiredTenant {
		return errors.New("assume role ARN is required")
	}

	return nil
}

func (o *TokenOptions) Key() string {
	ss := []string{
		Namespace,
		o.AccessKeyID,
		o.Region,
		o.Cluster,
		o.AssumeRoleARN,
	}
	if o.AssumeRoleARN == "" {
		ss[len(ss)-1] = "self"
	}

	return strings.Join(ss, "_")
}

// GetToken retrieves a token from cache or remote.
func GetToken(ctx context.Context, opts TokenOptions, cacher cache.Cache) (*token.Token, error) {
	logger := klog.LoggerWithName(klog.Background(), Namespace)

	err := opts.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Retrieve the token from cache.
	ck := opts.Key()
	if cacher != nil {
		bs, err := cacher.Get(ctx, ck)
		if err != nil && !errors.Is(err, cache.ErrEntryNotFound) {
			logger.Error(err, "error retrieving token from cache")
		}

		if len(bs) != 0 {
			var tk token.Token
			if err = tk.UnmarshalBinary(bs); err == nil {
				if !tk.Expired() {
					return &tk, nil
				}
			}

			if err != nil {
				logger.Error(err, "error unmarshalling cached token")
			}
		}
	}

	// Request the token from remote.
	tk, err := getToken(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting security token: %w", err)
	}

	// Save the token into cache.
	if cacher != nil {
		bs, err := tk.MarshalBinary()
		if err != nil {
			logger.Error(err, "error marshaling requested token")
		}

		if len(bs) != 0 {
			err = cacher.Set(ctx, ck, bs)
			if err != nil {
				logger.Error(err, "error saving token to cache")
			}
		}
	}

	return tk, nil
}

const (
	requestClusterIDHeader = "x-k8s-aws-id"
	requestPresignedParam  = 60

	presignedURLExpiration = 15 * time.Minute

	tokenPrefix = "k8s-aws-v1."
)

// getToken returns the token, inspired by
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/6c197aebdbe1d543f4dff5fee6ae32e71020313b/pkg/token/token.go#L336.
func getToken(ctx context.Context, opts TokenOptions) (*token.Token, error) {
	logger := klog.LoggerWithName(klog.Background(), Namespace)

	sess, err := session.NewSession(
		aws.NewConfig().
			WithCredentials(credentials.NewStaticCredentials(opts.AccessKeyID, opts.SecretAccessKey, "")).
			WithLogger(awsLogger(logger.V(5))).
			WithRegion(opts.Region).
			WithLogLevel(aws.LogDebug),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating session: %w", err)
	}

	api := sts.New(sess)
	if opts.AssumeRoleARN != "" {
		api = sts.New(sess,
			aws.NewConfig().
				WithCredentials(stscreds.NewCredentials(sess, opts.AssumeRoleARN)),
		)
	}

	// Generate sts:GetCallerIdentity request and add our custom cluster ID header.
	req, _ := api.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	req.HTTPRequest.Header.Add(requestClusterIDHeader, opts.Cluster)

	// Sign the request.  The expires parameter (sets the x-amz-expires header) is
	// currently ignored by STS, and the token expires 15 minutes after the x-amz-date
	// timestamp regardless.  We set it to 60 seconds for backwards compatibility (the
	// parameter is a required argument to Presign(), and authenticators 0.3.0 and older are expecting a value between
	// 0 and 60 on the server side).
	// https://github.com/aws/aws-sdk-go/issues/2167
	req.SetContext(ctx)

	presignedURLString, err := req.Presign(requestPresignedParam)
	if err != nil {
		return nil, err
	}

	// Set token expiration to 1 minute before the presigned URL expires for some cushion.
	tk := &token.Token{
		Expiration: time.Now().Local().Add(presignedURLExpiration - 1*time.Minute),
		Value:      tokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString)),
	}

	return tk, nil
}

type awsLogger klog.Logger

func (l awsLogger) Log(args ...any) {
	klog.Logger(l).Info(fmt.Sprint(args...))
}
