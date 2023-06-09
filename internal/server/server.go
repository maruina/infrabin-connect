package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	infrabinv1 "github.com/maruina/infrabin-connect/gen/infrabin/v1" // generated by protoc-gen-go
	"github.com/maruina/infrabin-connect/internal/aws"
)

const AWSAssumeRoleSessionName = "go-infrabin-aws-assume-role-endpoint"

type InfrabinServer struct {
	STSClient            aws.STSApi
	ProxyAllowedURLRegex string
	ProxyHTTPTimeout     time.Duration
}

func (s *InfrabinServer) Headers(ctx context.Context, req *connect.Request[infrabinv1.HeadersRequest]) (*connect.Response[infrabinv1.HeadersResponse], error) {
	res := connect.NewResponse(&infrabinv1.HeadersResponse{
		Headers: map[string]string{},
	})
	for k, vv := range req.Header() {
		for _, v := range vv {
			res.Msg.Headers[k] = v
		}
	}
	return res, nil
}

func (s *InfrabinServer) Env(ctx context.Context, req *connect.Request[infrabinv1.EnvRequest]) (*connect.Response[infrabinv1.EnvResponse], error) {
	res := connect.NewResponse(&infrabinv1.EnvResponse{
		Environment: map[string]string{},
	})
	if req.Msg.Key == "" {
		for _, item := range os.Environ() {
			a := strings.Split(item, "=")
			// Filter our any environment variable which is not "key=value"
			if len(a) == 2 {
				res.Msg.Environment[a[0]] = a[1]
			}
		}
		return res, nil
	}

	value, ok := os.LookupEnv(req.Msg.Key)
	if !ok {
		// If the environment variable does no exist return 404
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	res.Msg.Environment[req.Msg.Key] = value
	return res, nil
}

func (s *InfrabinServer) Root(ctx context.Context, req *connect.Request[infrabinv1.RootRequest]) (*connect.Response[infrabinv1.RootResponse], error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&infrabinv1.RootResponse{
		Hostname: hostname,
	})

	return res, nil
}

func (s *InfrabinServer) Delay(ctx context.Context, req *connect.Request[infrabinv1.DelayRequest]) (*connect.Response[infrabinv1.DelayResponse], error) {
	time.Sleep(req.Msg.Duration.AsDuration())

	res := connect.NewResponse(&infrabinv1.DelayResponse{})

	return res, nil
}

func (s *InfrabinServer) Proxy(ctx context.Context, req *connect.Request[infrabinv1.ProxyRequest]) (*connect.Response[infrabinv1.ProxyResponse], error) {
	// Compile the regexp
	re, err := regexp.Compile(s.ProxyAllowedURLRegex)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if the target URL is allowed
	if !re.MatchString(req.Msg.Url) {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	outboundReq, err := http.NewRequestWithContext(ctx, req.Msg.Method, req.Msg.Url, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for k, v := range req.Msg.Headers {
		outboundReq.Header.Set(k, v)
	}

	// Send http request
	client := http.Client{Timeout: 5 * time.Second}
	outboundRes, err := client.Do(outboundReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Read request body and close it
	_, err = io.ReadAll(outboundRes.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err = outboundRes.Body.Close(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&infrabinv1.ProxyResponse{
		StatusCode: int32(outboundRes.StatusCode),
		Headers:    make(map[string]string),
	})
	for k, vv := range outboundRes.Header {
		for _, v := range vv {
			res.Msg.Headers[k] = v
		}
	}
	return res, nil
}

func (s *InfrabinServer) AWSAssumeRole(ctx context.Context, req *connect.Request[infrabinv1.AWSAssumeRoleRequest]) (*connect.Response[infrabinv1.AWSAssumeRoleResponse], error) {
	if req.Msg.Role == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	assumedRoleId, err := aws.STSAssumeRole(ctx, s.STSClient, req.Msg.Role, AWSAssumeRoleSessionName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&infrabinv1.AWSAssumeRoleResponse{
		AssumedRoleId: assumedRoleId,
	})
	return res, nil
}

func (s *InfrabinServer) AWSGetCallerIdentity(ctx context.Context, req *connect.Request[infrabinv1.AWSGetCallerIdentityRequest]) (*connect.Response[infrabinv1.AWSGetCallerIdentityResponse], error) {
	stsRes, err := aws.STSGetCallerIdentity(ctx, s.STSClient)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&infrabinv1.AWSGetCallerIdentityResponse{
		Account: *stsRes.Account,
		Arn:     *stsRes.Arn,
		UserId:  *stsRes.UserId,
	})
	return res, nil
}
