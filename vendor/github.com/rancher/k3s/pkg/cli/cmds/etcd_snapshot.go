package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const EtcdSnapshotCommand = "etcd-snapshot"

func NewEtcdSnapshotCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            EtcdSnapshotCommand,
		Usage:           "Trigger an immediate etcd snapshot",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Flags: []cli.Flag{
			DebugFlag,
			LogFile,
			AlsoLogToStderr,
			cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
				Destination: &ServerConfig.DataDir,
			},
			&cli.StringFlag{
				Name:        "name",
				Usage:       "(db) Set the base name of the etcd on-demand snapshot (appended with UNIX timestamp).",
				Destination: &ServerConfig.EtcdSnapshotName,
				Value:       "on-demand",
			},
			&cli.StringFlag{
				Name:        "dir",
				Usage:       "(db) Directory to save etcd on-demand snapshot. (default: ${data-dir}/db/snapshots)",
				Destination: &ServerConfig.EtcdSnapshotDir,
			},
			&cli.BoolFlag{
				Name:        "s3",
				Usage:       "(db) Enable backup to S3",
				Destination: &ServerConfig.EtcdS3,
			},
			&cli.StringFlag{
				Name:        "s3-endpoint",
				Usage:       "(db) S3 endpoint url",
				Destination: &ServerConfig.EtcdS3Endpoint,
				Value:       "s3.amazonaws.com",
			},
			&cli.StringFlag{
				Name:        "s3-endpoint-ca",
				Usage:       "(db) S3 custom CA cert to connect to S3 endpoint",
				Destination: &ServerConfig.EtcdS3EndpointCA,
			},
			&cli.BoolFlag{
				Name:        "s3-skip-ssl-verify",
				Usage:       "(db) Disables S3 SSL certificate validation",
				Destination: &ServerConfig.EtcdS3SkipSSLVerify,
			},
			&cli.StringFlag{
				Name:        "s3-access-key",
				Usage:       "(db) S3 access key",
				EnvVar:      "AWS_ACCESS_KEY_ID",
				Destination: &ServerConfig.EtcdS3AccessKey,
			},
			&cli.StringFlag{
				Name:        "s3-secret-key",
				Usage:       "(db) S3 secret key",
				EnvVar:      "AWS_SECRET_ACCESS_KEY",
				Destination: &ServerConfig.EtcdS3SecretKey,
			},
			&cli.StringFlag{
				Name:        "s3-bucket",
				Usage:       "(db) S3 bucket name",
				Destination: &ServerConfig.EtcdS3BucketName,
			},
			&cli.StringFlag{
				Name:        "s3-region",
				Usage:       "(db) S3 region / bucket location (optional)",
				Destination: &ServerConfig.EtcdS3Region,
				Value:       "us-east-1",
			},
			&cli.StringFlag{
				Name:        "s3-folder",
				Usage:       "(db) S3 folder",
				Destination: &ServerConfig.EtcdS3Folder,
			},
		},
	}
}
