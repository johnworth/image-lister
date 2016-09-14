package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	images = flag.String("images", "", "Path to the list of images to pull.")
	repo   = flag.String("repo", "discoenv", "The Docker Hub repository to pull from.")
	tag    = flag.String("tag", "", "The tag to use when pulling the images.")
	uri    = flag.String("docker-uri", "unix:///var/run/docker.sock", "The docker client URI to use.")
)

// ReadLines parses a []byte into a []string based on newlines.
func ReadLines(content []byte) []string {
	var lines []string
	reader := bytes.NewReader(content)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// ReadImages reads in file and returns a []string of image names.
func ReadImages(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	filebytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return ReadLines(filebytes), nil
}

// OutputMap is contains the info that is printed to stdout.
type OutputMap struct {
	Hostname string
	Date     string
	Images   []types.Image
}

func main() {
	flag.Parse()

	if *images == "" {
		log.Fatal("--images must be set")
	}

	if *tag == "" {
		log.Fatal("--tag must be set")
	}

	var (
		err      error
		hostname string
		date     string
	)

	if hostname, err = os.Hostname(); err != nil {
		hostname = ""
	}

	date = time.Now().Format("2006-01-02T15:04:05-07:00")

	output := &OutputMap{
		Hostname: hostname,
		Date:     date,
	}

	ctx := context.Background()

	imageList, err := ReadImages(*images)
	if err != nil {
		log.Fatal(err)
	}

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	d, err := client.NewClient(*uri, "v1.22", nil, defaultHeaders)
	if err != nil {
		log.Fatalf("Error creating docker client: %s", err)
	}

	var (
		ref      string
		repoTags []string
		body     io.ReadCloser
	)
	for _, img := range imageList {
		ref = fmt.Sprintf("%s/%s:%s", *repo, img, *tag)
		repoTags = append(repoTags, ref)

		body, err = d.ImagePull(ctx, ref, types.ImagePullOptions{})
		defer body.Close()
		if err != nil {
			log.Fatal(err)
		}

		if _, err = io.Copy(os.Stderr, body); err != nil {
			log.Fatal(err)
		}
	}

	listedImages, err := d.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		log.Fatal(err)
	}

	// Only include an image in the output if one of its RepoTags is included
	// in the list of images that the user specified on the command-line.
	for _, rt := range repoTags {
		for _, li := range listedImages {
			for _, listedRepoTag := range li.RepoTags {
				if listedRepoTag == rt {
					output.Images = append(output.Images, li)
				}
			}
		}
	}

	imgJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %s", err)
	}

	if _, err = os.Stdout.Write(imgJSON); err != nil {
		log.Fatal(err)
	}
}
