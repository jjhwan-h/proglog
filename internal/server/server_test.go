package server

import (
	"context"
	"net"
	"os"
	"testing"

	api "github.com/jjhwan-h/proglog/api/v1"
	"github.com/jjhwan-h/proglog/internal/config"
	"github.com/jjhwan-h/proglog/internal/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T,
		rootClient api.LogClient,
		nobodyClient api.LogClient,
		config *Config,
	){
		"produce/consume a message to/from the log succeeds": testProduceConsume,
		"produce/consume stream succeeds":                    testProduceConsumeStream,
		"consume past log boundary fails":                    testConsumePastBoundary,
	} {
		t.Run(scenario, func(t *testing.T) {
			rootClient, nobodyClient, config, teardown := setupTest(t, nil)
			defer teardown()
			fn(t, rootClient, nobodyClient, config)
		})
	}
}

// func setupTest(t *testing.T, fn func(*Config)) (
// 	client api.LogClient,
// 	cfg *Config,
// 	teardown func(),
// ) {
// 	t.Helper()

// 	l, err := net.Listen("tcp", "127.0.0.1:0")
// 	require.NoError(t, err)

// 	clientTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
// 		CertFile: config.ClientCertFile,
// 		KeyFile:  config.ClientKeyFile,
// 		CAFile:   config.CAFile,
// 	})
// 	require.NoError(t, err)

// 	clientCreds := credentials.NewTLS(clientTLSConfig)
// 	cc, err := grpc.NewClient(
// 		l.Addr().String(),
// 		grpc.WithTransportCredentials(clientCreds),
// 	)
// 	require.NoError(t, err)

// 	client = api.NewLogClient(cc)

// 	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
// 		CertFile:      config.ServerCertFile,
// 		KeyFile:       config.ServerKeyFile,
// 		CAFile:        config.CAFile,
// 		ServerAddress: l.Addr().String(),
// 		Server:        true,
// 	})
// 	require.NoError(t, err)
// 	serverCreds := credentials.NewTLS(serverTLSConfig)

// 	dir, err := os.MkdirTemp("", "server-test")
// 	require.NoError(t, err)

// 	clog, err := log.NewLog(dir, log.Config{})
// 	require.NoError(t, err)

// 	cfg = &Config{
// 		CommitLog: clog,
// 	}
// 	if fn != nil {
// 		fn(cfg)
// 	}
// 	server, err := NewGRPCServer(cfg, grpc.Creds(serverCreds))
// 	require.NoError(t, err)

// 	go func() {
// 		server.Serve(l)
// 	}()

// 	return client, cfg, func() {
// 		server.Stop()
// 		cc.Close()
// 		l.Close()
// 		clog.Remove()
// 	}
// }

func setupTest(t *testing.T, fn func(*Config)) (
	rootClient api.LogClient,
	nobodyClient api.LogClient,
	cfg *Config,
	teardown func(),
) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	newClient := func(crtPath, keyPath string) (
		*grpc.ClientConn,
		api.LogClient,
		[]grpc.DialOption,
	) {
		tlsConfig, err := config.SetupTLSConfig(config.TLSConfig{
			CertFile: crtPath,
			KeyFile:  keyPath,
			CAFile:   config.CAFile,
			Server:   false,
		})
		require.NoError(t, err)
		tlsCreds := credentials.NewTLS(tlsConfig)
		opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
		conn, err := grpc.NewClient(l.Addr().String(), opts...)
		require.NoError(t, err)
		client := api.NewLogClient(conn)
		return conn, client, opts
	}

	// superuser: 생산과 소비 가능
	rootConn, rootClient, _ := newClient(
		config.RootClientCertFile,
		config.RootClientKeyFile,
	)

	// 아무런 권한 없음
	nobodyConn, nobodyClient, _ := newClient(
		config.NobodyClientCertFile,
		config.NobodyClientKeyFile,
	)

	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		ServerAddress: l.Addr().String(),
		Server:        true,
	})
	require.NoError(t, err)
	serverCreds := credentials.NewTLS(serverTLSConfig)

	dir, err := os.MkdirTemp("", "server-test")
	require.NoError(t, err)

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	cfg = &Config{
		CommitLog: clog,
	}
	if fn != nil {
		fn(cfg)
	}
	server, err := NewGRPCServer(cfg, grpc.Creds(serverCreds))
	require.NoError(t, err)

	go func() {
		server.Serve(l)
	}()

	return rootClient, nobodyClient, cfg, func() {
		server.Stop()
		rootConn.Close()
		nobodyConn.Close()
		l.Close()
	}
}

/*
클라이언트와 서버를 이용하여 생산과 소비가 이루어지는지,
로그에 레코드를 추가하고 다시 소비하는지를 확인
*/
func testProduceConsume(t *testing.T, client api.LogClient, _ api.LogClient, config *Config) {
	ctx := context.Background()

	want := &api.Record{
		Value: []byte("hello world"),
	}

	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: want,
		},
	)
	require.NoError(t, err)

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset,
	})
	require.NoError(t, err)
	require.Equal(t, want.Value, consume.Record.Value)
	require.Equal(t, want.Offset, consume.Record.Offset)
}

/*
클라이언트가 로그의 범위를 벗어난 시도를 할 때,
서버가 api.ErrOffsetOutOfRange()에러를 회신하는지 확인
*/
func testConsumePastBoundary(
	t *testing.T,
	client api.LogClient,
	_ api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: &api.Record{
			Value: []byte("hello world"),
		},
	})
	require.NoError(t, err)
	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset + 1,
	})
	if consume != nil {
		t.Fatal("consume not nil")
	}
	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{}.GRPCStatus().Err())
	if got != want {
		t.Fatalf("got err: %v, want: %v", got, want)
	}
}

/*
스트림을 통한 생산과 소비를 확인
*/
func testProduceConsumeStream(
	t *testing.T,
	client api.LogClient,
	_ api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	records := []*api.Record{{
		Value:  []byte("first message"),
		Offset: 0,
	}}

	{
		stream, err := client.ProduceStream(ctx)
		require.NoError(t, err)

		for offset, record := range records {
			err = stream.Send(&api.ProduceRequest{
				Record: record,
			})
			require.NoError(t, err)
			res, err := stream.Recv()
			require.NoError(t, err)
			if res.Offset != uint64(offset) {
				t.Fatalf(
					"got offset: %d, want: %d",
					res.Offset,
					offset,
				)
			}
		}
	}
	{
		stream, err := client.ConsumeStream(
			ctx,
			&api.ConsumeRequest{Offset: 0},
		)
		require.NoError(t, err)

		for i, record := range records {
			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, res.Record, &api.Record{
				Value:  record.Value,
				Offset: uint64(i),
			})
		}
	}
}
