package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	version string
)

func main() {
	targetDirPath := flag.String("d", "", "The directory to search for .NET files")
	fileExtsCsv := flag.String("e", ".dll", "Comma separated list of file extensions to search for")
	outputDirPath := flag.String("o", "", "The output directory. Creates a new directory if not specified")
	respectFileCase := flag.Bool("respect-file-case", false, "Respect filenames' case when matching their extensions")
	noIlspyErrors := flag.Bool("no-ilspy-errors", false, "Exit if ILSpy fails to decompile a file")
	scanRecursively := flag.Bool("r", false, "Scan recursively")
	numDecompilers := flag.Int("num-workers", runtime.NumCPU(), "Number of .NET decompiler instances to run concurrently")
	allowDupelicateFiles := flag.Bool("allow-duplicates", false, "Decompile file even if its hash has already been encountered")
	ilspycmdPath := flag.String("ilspy", "ilspycmd", "The 'ilspycmd' binary to use")

	flag.Parse()

	_, err := exec.LookPath(*ilspycmdPath)
	if err != nil {
		log.Fatalf("failed to find the specified 'ilspycmd' binary ('%s') - %s",
			*ilspycmdPath, err.Error())
	}

	if len(*targetDirPath) == 0 {
		log.Fatal("please specify a target directory path")
	}

	if len(*fileExtsCsv) == 0 {
		log.Fatal("please specify a comma separated list of file extensions")
	}

	if len(*outputDirPath) == 0 {
		*outputDirPath = filepath.Base(*targetDirPath)
	}

	if !*respectFileCase {
		*fileExtsCsv = strings.ToLower(*fileExtsCsv)
	}

	fileExts := strings.Split(*fileExtsCsv, ",")

	absTargetDirPath, err := filepath.Abs(*targetDirPath)
	if err != nil {
		log.Fatalf("failed to determine absolute path of target directory - %s", err.Error())
	}

	start := time.Now()

	d := newDoer(*numDecompilers)

	fileHashesToDecompiledPaths := make(map[string]string)

	err = filepath.Walk(*targetDirPath,
		func(filePath string, info os.FileInfo, err error) error {
			// Gotta check the error provided by the last call.
			if err != nil {
				return err
			}

			if !*scanRecursively {
				if filepath.Dir(filePath) != absTargetDirPath {
					return filepath.SkipDir
				}
			}

			if info.IsDir() {
				return nil
			}

			filename := info.Name()
			if !*respectFileCase {
				filename = strings.ToLower(filename)
			}

			matchedExtension := false
			for i := range fileExts {
				if strings.HasSuffix(filename, fileExts[i]) {
					matchedExtension = true
					break
				}
			}
			if !matchedExtension {
				return nil
			}

			fileSha256str, err := hashFile(filePath, sha256.New())
			if err != nil {
				return fmt.Errorf("failed to hash file '%s' - %s", filePath, err.Error())
			}

			finalOutputDirPath := finalOutputDirCalc{
				searchAbsPath:     absTargetDirPath,
				targetFileAbsPath: filePath,
				outputDirPath:     *outputDirPath,
			}.get()

			if !*allowDupelicateFiles {
				if existingPath, containsFileHash := fileHashesToDecompiledPaths[fileSha256str]; containsFileHash {
					err = os.MkdirAll(finalOutputDirPath, 0700)
					if err != nil {
						return fmt.Errorf("failed to create directory for ignored .NET file '%s' - %s",
							filePath, err.Error())
					}
					err = ioutil.WriteFile(filepath.Join(finalOutputDirPath, "ignored.log"),
						[]byte(fmt.Sprintf("file has already been decomipled to '%s', hash of file is %s\n",
							existingPath, fileSha256str)),
						0600)
					if err != nil {
						return fmt.Errorf("failed to create ignored log for ignored .NET file '%s' - %s",
							filePath, err.Error())
					}
					return nil
				}

				fileHashesToDecompiledPaths[fileSha256str] = finalOutputDirPath
			}

			d.queue(func() error {
				err := decompileNETFile(decompileNETInfo{
					ilspycmdPath:       *ilspycmdPath,
					filePath:           filePath,
					finalOutputDirPath: finalOutputDirPath,
				})
				if err != nil {
					if _, ok := err.(*ilspyError); ok {
						if *noIlspyErrors {
							return err
						}
						ioutil.WriteFile(filepath.Join(finalOutputDirPath, "decompile-failure.log"),
							[]byte(err.Error()),
							0600)
						log.Printf("[warn] %s", err.Error())
						return nil
					}
					return fmt.Errorf("failed to decompile '%s' - %s", filePath, err.Error())
				}

				err = ioutil.WriteFile(filepath.Join(finalOutputDirPath, "hash.txt"),
					[]byte(fmt.Sprintf("%s\n", fileSha256str)),
					0600)
				if err != nil {
					return fmt.Errorf("failed to write hash file for '%s' - %s",
						filePath, fileSha256str)
				}

				log.Printf("decompiled '%s' to '%s'", filePath, finalOutputDirPath)

				return nil
			})

			return nil
		})
	if err != nil {
		log.Println(err)
	}

	err = d.wait()
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Printf("finished after %s", time.Since(start).String())
}

type finalOutputDirCalc struct {
	searchAbsPath     string
	targetFileAbsPath string
	outputDirPath     string
}

func (o finalOutputDirCalc) get() string {
	return filepath.Join(o.outputDirPath, strings.TrimPrefix(o.targetFileAbsPath, o.searchAbsPath))
}

type decompileNETInfo struct {
	ilspycmdPath       string
	filePath           string
	finalOutputDirPath string
}

func decompileNETFile(info decompileNETInfo) error {
	err := os.MkdirAll(info.finalOutputDirPath, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output subdirectory - %s", err)
	}

	raw, err := exec.Command("ilspycmd", info.filePath, "-p", "-o", info.finalOutputDirPath).CombinedOutput()
	if err != nil {
		return &ilspyError{
			err: fmt.Sprintf("failed to decompile .NET file '%s' - %s - %s",
				info.filePath, err.Error(), raw),
		}
	}

	return nil
}

type ilspyError struct {
	err string
}

func (o ilspyError) Error() string {
	return o.err
}

func newDoer(numWorkers int) *doer {
	pool := make(chan int, numWorkers)
	for i := 0; i < numWorkers; i++ {
		pool <- i
	}

	return &doer{
		pool:   pool,
		failed: make(chan error, 1),
		dead:   make(chan struct{}),
		wg:     &sync.WaitGroup{},
	}
}

type doer struct {
	pool   chan int
	failed chan error
	dead   chan struct{}
	wg     *sync.WaitGroup
}

func (o *doer) queue(fn func() error) {
	o.wg.Add(1)
	go func() {
		select {
		case workerID := <-o.pool:
			err := fn()
			if err != nil {
				select {
				case o.failed <- err:
				default:
					close(o.dead)
				}
				o.wg.Done()
				return
			}
			o.wg.Done()
			o.pool <- workerID
		case <-o.dead:
			return
		}
	}()
}

func (o *doer) wait() error {
	o.wg.Wait()
	select {
	case err := <-o.failed:
		return err
	default:
		return nil
	}
}

func hashFile(filePath string, hasher hash.Hash) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(hasher, f)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
