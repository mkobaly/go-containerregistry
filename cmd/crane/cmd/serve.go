// Copyright 2023 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/pkg/registry"
)

func newCmdRegistry() *cobra.Command {
	cmd := &cobra.Command{
		Use: "registry",
	}
	cmd.AddCommand(newCmdServe())
	return cmd
}

func newCmdServe() *cobra.Command {
	//var disk bool
	var disk string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve an in-memory or on-disk registry",
		Long: `This sub-command serves an in-memory or on-disk registry implementation

PORT: randomly chosen or you can set via environment variable ($PORT)

For on-disk
-------------
Use the hidden parameter --blobs-to-disk ./images

The command blocks while the server accepts pushes and pulls.

For in-memory registry, when the process exits, pushed data is lost.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			port := os.Getenv("PORT")
			if port == "" {
				port = "0"
			}
			listener, err := net.Listen("tcp", ":"+port)
			if err != nil {
				log.Fatalln(err)
			}
			porti := listener.Addr().(*net.TCPAddr).Port
			port = fmt.Sprintf("%d", porti)

			bh := registry.NewInMemoryBlobHandler()
			if disk != "" {
				if _, err := os.Stat(disk); errors.Is(err, os.ErrNotExist) {
					err := os.Mkdir(disk, os.ModePerm)
					if err != nil {
						log.Fatalln(err)
					}
				}
				bh = registry.NewDiskBlobHandler(disk)
				log.Printf("serving on-disk registry at %s", disk)
			}

			s := &http.Server{
				ReadHeaderTimeout: 5 * time.Second, // prevent slowloris, quiet linter
				Handler:           registry.New(registry.WithBlobHandler(bh), registry.WithDiskManifest(disk)),
			}
			log.Printf("serving on port %s", port)

			errCh := make(chan error)
			go func() { errCh <- s.Serve(listener) }()

			<-ctx.Done()
			log.Println("shutting down...")
			if err := s.Shutdown(ctx); err != nil {
				return err
			}

			if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&disk, "blobs-to-disk", "", "Store blobs on disk at the given path")
	//cmd.Flags().BoolVar(&disk, "blobs-to-disk", false, "Store blobs on disk")
	cmd.Flags().MarkHidden("blobs-to-disk")
	return cmd
}
