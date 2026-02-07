// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tencent/goom/internal/hack"
)

const osWindows = "windows"

func init() {
	http.DefaultTransport = &userAgentTransport{http.DefaultTransport}
}

// Run runs the "go" tool of the provided Go version.
func Run(version string, logHandler func(log string), args ...string) error {
	log.SetFlags(0)

	root, err := goroot(version)
	if err != nil {
		log.Fatalf("%s: %v", version, err)
	}

	if err := install(root, version); err != nil {
		log.Fatalf("%s: download failed: %v", version, err)
	}

	if _, err := os.Stat(filepath.Join(root, unpackedOkay)); err != nil {
		log.Fatalf("%s: not downloaded. Run '%s download' to install to %v", version, version, root)
	}

	return runGo(root, logHandler, args)
}

func runGo(root string, logHandler func(log string), args []string) error {
	gobin := filepath.Join(root, "bin", "go"+exe())
	cmd := exec.Command(gobin, args...)
	// cmd.Stdin = os.Stdin
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	newPath := filepath.Join(root, "bin")
	if p := os.Getenv("PATH"); p != "" {
		newPath += string(filepath.ListSeparator) + p
	}
	cmd.Env = dedupEnv(caseInsensitiveEnv, append(os.Environ(), "GOROOT="+root, "PATH="+newPath))

	handleSignals()

	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("cmd exec failed: %v", err)
	}
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(cmdReader)
	done := make(chan bool)
	go func() {
		for scanner.Scan() {
			logs := scanner.Text()
			if logHandler != nil {
				logHandler(logs)
			}
		}
		done <- true
	}()

	if err := cmd.Start(); err != nil {
		log.Fatalf("cmd exec failed: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	<-done
	return nil
}

// install installs a version of Go to the named target directory, creating the
// directory as needed.
func install(targetDir, version string) error {
	if _, err := os.Stat(filepath.Join(targetDir, unpackedOkay)); err == nil {
		log.Printf("%s: already downloaded in %v", version, targetDir)
		return nil
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	goURL := versionArchiveURL(version)
	res, err := http.Head(goURL)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no binary release of %v for %v/%v at %v", version, getOS(), runtime.GOARCH, goURL)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %v checking size of %v", http.StatusText(res.StatusCode), goURL)
	}
	base := path.Base(goURL)
	archiveFile := filepath.Join(targetDir, base)
	if fi, err2 := os.Stat(archiveFile); err2 != nil || fi.Size() != res.ContentLength {
		if err2 != nil && !os.IsNotExist(err2) {
			// Something weird. Don't try to download.
			return err2
		}
		if err3 := copyFromURL(archiveFile, goURL, res.ContentLength); err3 != nil {
			return fmt.Errorf("error downloading %v: %v", goURL, err3)
		}
		fi, err2 = os.Stat(archiveFile)
		if err2 != nil {
			return err2
		}
		if fi.Size() != res.ContentLength {
			return fmt.Errorf("downloaded file %s size %v doesn't match server size %v",
				archiveFile, fi.Size(), res.ContentLength)
		}
	}
	wantSHA, err := slurpURLToString(goURL + ".sha256")
	if err != nil {
		return err
	}
	if err := verifySHA256(archiveFile, strings.TrimSpace(wantSHA)); err != nil {
		return fmt.Errorf("error verifying SHA256 of %v: %v", archiveFile, err)
	}
	log.Printf("Unpacking %v ...", archiveFile)
	if err := unpackArchive(targetDir, archiveFile); err != nil {
		return fmt.Errorf("extracting archive %v: %v", archiveFile, err)
	}
	if err := ioutil.WriteFile(filepath.Join(targetDir, unpackedOkay), nil, 0644); err != nil {
		return err
	}
	log.Printf("Success. You may now run '%v'", version)
	return nil
}

// unpackArchive unpacks the provided archive zip or tar.gz file to targetDir,
// removing the "go/" prefix from file entries.
func unpackArchive(targetDir, archiveFile string) error {
	switch {
	case strings.HasSuffix(archiveFile, ".zip"):
		return unpackZip(targetDir, archiveFile)
	case strings.HasSuffix(archiveFile, ".tar.gz"):
		return unpackTarGz(targetDir, archiveFile)
	default:
		return errors.New("unsupported archive file")
	}
}

// unpackTarGz is the tar.gz implementation of unpackArchive.
func unpackTarGz(targetDir, archiveFile string) error {
	r, err := os.Open(archiveFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = r.Close()
	}()
	madeDir := map[string]bool{}
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !validRelPath(f.Name) {
			return fmt.Errorf("tar file contained invalid name %q", f.Name)
		}
		rel := filepath.FromSlash(strings.TrimPrefix(f.Name, "go/"))
		abs := filepath.Join(targetDir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			n, err := io.Copy(wf, tr)
			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			if !f.ModTime.IsZero() {
				if err := os.Chtimes(abs, f.ModTime, f.ModTime); err != nil {
					// benign error. Gerrit doesn't even set the
					// modtime in these, and we don't end up relying
					// on it anywhere (the gomote push command relies
					// on digests only), so this is a little pointless
					// for now.
					log.Printf("error changing modtime: %v", err)
				}
			}
		case mode.IsDir():
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			madeDir[abs] = true
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}

// unpackZip is the zip implementation of unpackArchive.
func unpackZip(targetDir, archiveFile string) error {
	zr, err := zip.OpenReader(archiveFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = zr.Close()
	}()

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "go/")

		outpath := filepath.Join(targetDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outpath, 0755); err != nil {
				return err
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		// File
		if err2 := os.MkdirAll(filepath.Dir(outpath), 0755); err2 != nil {
			return err2
		}
		out, err := os.OpenFile(outpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(out, rc)
		_ = rc.Close()
		if err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

// verifySHA256 reports whether the named file has contents with
// SHA-256 of the given wantHex value.
func verifySHA256(file, wantHex string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != wantHex {
		return fmt.Errorf("%s corrupt? does not have expected SHA-256 of %v", file, wantHex)
	}
	return nil
}

// slurpURLToString downloads the given URL and returns it as a string.
func slurpURLToString(uRL string) (string, error) {
	res, err := http.Get(uRL)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %v", uRL, res.Status)
	}
	slurp, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("reading %s: %v", uRL, err)
	}
	return string(slurp), nil
}

// copyFromURL downloads srcURL to dstFile with best-effort resume support.
func copyFromURL(dstFile, srcURL string, expectedSize int64) (err error) {
	var startOffset int64
	if fi, statErr := os.Stat(dstFile); statErr == nil {
		startOffset = fi.Size()
		if expectedSize > 0 && startOffset > expectedSize {
			_ = os.Remove(dstFile)
			startOffset = 0
		}
		if expectedSize > 0 && startOffset == expectedSize {
			return nil
		}
	}

	f, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	if startOffset > 0 {
		if _, err = f.Seek(startOffset, io.SeekStart); err != nil {
			return err
		}
	}
	c := &http.Client{
		Transport: &userAgentTransport{&http.Transport{
			// It's already compressed. Prefer accurate ContentLength.
			// (Not that GCS would try to compress it, though)
			DisableCompression: true,
			DisableKeepAlives:  true,
			Proxy:              http.ProxyFromEnvironment,
		}},
	}
	req, err := http.NewRequest(http.MethodGet, srcURL, nil)
	if err != nil {
		return err
	}
	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return errors.New(res.Status)
	}
	if startOffset > 0 && res.StatusCode == http.StatusOK {
		// Server ignored Range; restart from scratch.
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		startOffset = 0
	}
	total := expectedSize
	if total <= 0 {
		total = res.ContentLength + startOffset
	}
	pw := &progressWriter{w: f, total: total, n: startOffset}
	n, err := io.Copy(pw, res.Body)
	if err != nil {
		return err
	}
	if res.ContentLength != -1 && res.ContentLength != n {
		return fmt.Errorf("copied %v bytes; expected %v", n, res.ContentLength)
	}
	pw.update() // 100%
	return f.Close()
}

type progressWriter struct {
	w     io.Writer
	n     int64
	total int64
	last  time.Time
}

func (p *progressWriter) update() {
	if p.total <= 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Downloaded %d bytes ...\n", p.n)
		return
	}
	end := " ..."
	if p.n == p.total {
		end = ""
	}
	_, _ = fmt.Fprintf(os.Stderr, "Downloaded %5.1f%% (%*d / %d bytes)%s\n",
		(100.0*float64(p.n))/float64(p.total),
		ndigits(p.total), p.n, p.total, end)
}

func ndigits(i int64) int {
	var n int
	for ; i != 0; i /= 10 {
		n++
	}
	return n
}

func (p *progressWriter) Write(buf []byte) (n int, err error) {
	n, err = p.w.Write(buf)
	p.n += int64(n)
	if now := time.Now(); now.Unix() != p.last.Unix() {
		p.update()
		p.last = now
	}
	return
}

// getOS returns runtime.GOOS. It exists as a function just for lazy
// testing of the Windows zip path when running on Linux/Darwin.
func getOS() string {
	return runtime.GOOS
}

// versionArchiveURL returns the zip or tar.gz URL of the given Go version.
func versionArchiveURL(version string) string {
	goos := getOS()

	ext := ".tar.gz"
	if goos == osWindows {
		ext = ".zip"
	}
	arch := runtime.GOARCH
	if goos == "linux" && runtime.GOARCH == "arm" {
		arch = "armv6l"
	}
	return "https://dl.google.com/go/" + version + "." + goos + "-" + arch + ext
}

const caseInsensitiveEnv = runtime.GOOS == osWindows

// unpackedOkay is a sentinel zero-byte file to indicate that the Go
// version was downloaded and unpacked successfully.
const unpackedOkay = ".unpacked-success"

func exe() string {
	if runtime.GOOS == osWindows {
		return ".exe"
	}
	return ""
}

func goroot(version string) (string, error) {
	home, err := homedir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}
	return filepath.Join(home, "sdk", version), nil
}

func homedir() (string, error) {
	// This could be replaced with os.UserHomeDir, but it was introduced too
	// recently, and we want this to work with go as packaged by Linux
	// distributions. Note that user.Current is not enough as it does not
	// prioritize $HOME. See also Issue 26463.
	switch getOS() {
	case "plan9":
		return "", fmt.Errorf("%q not yet supported", runtime.GOOS)
	case osWindows:
		if dir := os.Getenv("USERPROFILE"); dir != "" {
			return dir, nil
		}
		return "", errors.New("can't find user home directory; %USERPROFILE% is empty")
	default:
		if dir := os.Getenv("HOME"); dir != "" {
			return dir, nil
		}
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			return u.HomeDir, nil
		}
		return "", errors.New("can't find user home directory; $HOME is empty")
	}
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}

type userAgentTransport struct {
	rt http.RoundTripper
}

func (uat userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	version := runtime.Version()
	if strings.Contains(version, "devel") {
		// Strip the SHA hash and date. We don't want spaces or other tokens (see RFC2616 14.43)
		version = "devel"
	}
	r.Header.Set("User-Agent", "golang-x-build-version/"+version)
	return uat.rt.RoundTrip(r)
}

// dedupEnv returns a copy of env with any duplicates removed, in favor of
// later values.
// Items are expected to be on the normal environment "key=value" form.
// If caseInsensitive is true, the case of keys is ignored.
//
// This function is unnecessary when the binary is
// built with Go 1.9+, but keep it around for now until Go 1.8
// is no longer seen in the wild in common distros.
//
// This is copied verbatim from golang.org/x/build/envutil.Dedup at CL 10301
// (commit a91ae26).
func dedupEnv(caseInsensitive bool, env []string) []string {
	out := make([]string, 0, len(env))
	saw := map[string]int{} // to index in the array
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq < 1 {
			out = append(out, kv)
			continue
		}
		k := kv[:eq]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if dupIdx, isDup := saw[k]; isDup {
			out[dupIdx] = kv
		} else {
			saw[k] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

// nolint
func handleSignals() {
	// Ensure that signals intended for the child process are not handled by
	// this process' runtime (e.g. SIGQUIT). See issue #36976.
	signal.Notify(make(chan os.Signal), hack.SignalsToIgnore...)
}
