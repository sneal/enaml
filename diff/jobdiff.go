package diff

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/kr/pretty"
	"github.com/xchapter7x/enaml"
	"github.com/xchapter7x/enaml/pull"
)

func NewDiff(cacheDir string) *Diff {
	return &Diff{CacheDir: cacheDir}
}

type Diff struct {
	CacheDir string
}

func (s *Diff) ReleaseDiff(releaseURLA, releaseURLB string) (diffset []string, err error) {
	var filenameA string
	var filenameB string
	release := pull.NewRelease(s.CacheDir)
	if filenameA, err = release.Pull(releaseURLA); err == nil {
		if filenameB, err = release.Pull(releaseURLB); err == nil {
			objA := GetReleaseManifest(filenameA)
			objB := GetReleaseManifest(filenameB)
			diffset = pretty.Diff(objA, objB)
		}
	}
	return
}

func (s *Diff) JobDiffBetweenReleases(jobname, releaseURLA, releaseURLB string) (diffset []string, err error) {
	var (
		jobA      *tar.Reader
		jobB      *tar.Reader
		filenameA string
		filenameB string
		ok        bool
	)
	release := pull.NewRelease(s.CacheDir)
	filenameA, err = release.Pull(releaseURLA)

	if err != nil {
		err = fmt.Errorf("An error occurred downloading %s. %s", releaseURLA, err.Error())
		return
	}
	filenameB, err = release.Pull(releaseURLB)

	if err != nil {
		err = fmt.Errorf("An error occurred downloading %s. %s", releaseURLB, err.Error())
		return
	}

	if jobA, ok = ProcessReleaseArchive(filenameA)[jobname]; !ok {
		err = errors.New(fmt.Sprintf("could not find jobname %s in release A", jobname))
		return
	}

	if jobB, ok = ProcessReleaseArchive(filenameB)[jobname]; !ok {
		err = errors.New(fmt.Sprintf("could not find jobname %s in release B", jobname))
		return
	}
	bufA := new(bytes.Buffer)
	bufA.ReadFrom(jobA)
	bufB := new(bytes.Buffer)
	bufB.ReadFrom(jobB)
	diffset = JobPropertiesDiff(bufA.Bytes(), bufB.Bytes())
	return
}

func GetReleaseManifest(srcFile string) (releaseManifest enaml.ReleaseManifest) {
	f, err := os.Open(srcFile)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	tarReader := getTarballReader(f)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		name := header.Name

		switch header.Typeflag {
		case tar.TypeReg:
			if path.Base(name) == "release.MF" {
				if b, err := ioutil.ReadAll(tarReader); err == nil {
					releaseManifest = enaml.ReleaseManifest{}
					yaml.Unmarshal(b, &releaseManifest)
				}
			}
		}
	}
	return
}

func ProcessReleaseArchive(srcFile string) (jobs map[string]*tar.Reader) {
	jobs = make(map[string]*tar.Reader)
	f, err := os.Open(srcFile)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	tarReader := getTarballReader(f)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		name := header.Name

		switch header.Typeflag {
		case tar.TypeReg:
			if strings.HasPrefix(name, "./jobs/") {
				jobTarball := getTarballReader(tarReader)
				jobManifest := getJobManifestFromTarball(jobTarball)
				jobName := strings.Split(path.Base(name), ".")[0]
				jobs[jobName] = jobManifest
			}
		}
	}
	return
}

func getTarballReader(reader io.Reader) *tar.Reader {
	gzf, err := gzip.NewReader(reader)

	if err != nil {
		fmt.Println(err)
	}
	return tar.NewReader(gzf)
}

func getJobManifestFromTarball(jobTarball *tar.Reader) (res *tar.Reader) {
	var jobManifestFilename = "./job.MF"

	for {
		header, _ := jobTarball.Next()
		if header.Name == jobManifestFilename {
			res = jobTarball
			break
		}
	}
	return
}

func JobPropertiesDiff(a, b []byte) []string {
	var objA enaml.JobManifest
	var objB enaml.JobManifest
	yaml.Unmarshal(a, &objA)
	yaml.Unmarshal(b, &objB)
	mp := pretty.Diff(objA, objB)
	return mp
}
