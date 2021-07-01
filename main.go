package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func main() {
	namePtr := flag.String("name", "pg_db", "The name of the docker container, default pg_db")
	userPtr := flag.String("user", "postgres", "The username of the postgres user, default postgres")
	passPtr := flag.String("password", "postgres", "The password of the postgres user, default postgres")
	dbNamePtr := flag.String("dbName", "db", "The default database name, default db")
	persistPtr := flag.Bool("persist", false, "Should the database persist")

	flag.Parse()

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	imageName := "postgres:alpine"
	logFile, err := os.Create("pg_logs.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create logging file")
		logFile = os.Stdout
	}

	defer logFile.Close()

	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}

	io.Copy(logFile, out)

	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	persistPath := fmt.Sprintf("%s/.config/pg_new/%s", dirname, *namePtr)

	var mounts []mount.Mount
	if *persistPtr {
		os.MkdirAll(persistPath, 0755)

		mounts = []mount.Mount{{
			Type:   mount.TypeBind,
			Source: persistPath,
			Target: "/var/lib/postgresql/data",
		},
		}
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Env: []string{
			"POSTGRES_PASSWORD=" + *passPtr,
			"POSTGRES_USER=" + *userPtr,
			"POSTGRES_DB=" + *dbNamePtr,
		},
		ExposedPorts: nat.PortSet{
			"5432/tcp": struct{}{},
		},
	}, &container.HostConfig{
		Mounts: mounts,
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "5432",
				},
			},
		},
	},
		nil, nil, *namePtr)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	fmt.Printf("Started database container %s with ID %s\n", *namePtr, resp.ID)
	fmt.Println("Username " + *userPtr)
	fmt.Println("Password " + *passPtr)
	fmt.Println("Database " + *dbNamePtr)
	fmt.Printf("Persist %t\n", *persistPtr)

	fmt.Print("Press any key to stop container")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	if err := cli.ContainerStop(ctx, resp.ID, nil); err != nil {
		panic(err)
	}

	fmt.Println("Stopped container")

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}

	fmt.Println("Removed container")
}
