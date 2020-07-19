package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/schollz/progressbar/v2"
	"github.com/stephen-fox/filesearch"
	"github.com/stephen-fox/grp"
)

const (
	usageFormat = `finley - %s

usage: finley [options] directory-path/

[options]
%s`
)

var (
	version string
)

func main() {
	fileExtsCsv := flag.String("e", ".dll,.exe", "Comma separated list of file extensions to search for")
	outputDirPath := flag.String("o", "", "The output directory. Creates a new directory if not specified")
	respectFileCase := flag.Bool("respect-file-case", false, "Respect filenames' case when matching their extensions")
	noIlspyErrors := flag.Bool("no-ilspy-errors", false, "Exit if ILSpy fails to decompile a file")
	scanRecursively := flag.Bool("r", false, "Scan recursively")
	numDecompilers := flag.Int("num-workers", runtime.NumCPU(), "Number of .NET decompiler instances to run concurrently")
	allowDuplicateFiles := flag.Bool("allow-duplicates", false, "Decompile file even if its hash has already been encountered")
	ilspycmdPath := flag.String("ilspy", "ilspycmd", "The 'ilspycmd' binary to use")
	verbose := flag.Bool("v", false, "Display log messages rather than a progress bar")
	showVersion := flag.Bool("version", false, "Display the version number and exit")
	help := flag.Bool("h", false, "Display this help page")

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(1)
	}

	if *help {
		buff := bytes.NewBuffer(nil)
		flag.CommandLine.SetOutput(buff)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usageFormat,
			version, buff.String())
		os.Exit(1)
	}

	if flag.NArg() != 1 {
		log.Fatalf("please specify the directory to search as the final argument")
	}

	targetDirPath := flag.Arg(0)

	_, err := exec.LookPath(*ilspycmdPath)
	if err != nil {
		log.Fatalf("failed to find the specified 'ilspycmd' binary ('%s') - %s",
			*ilspycmdPath, err.Error())
	}

	if len(*fileExtsCsv) == 0 {
		log.Fatal("please specify a comma separated list of file extensions")
	}

	if len(*outputDirPath) == 0 {
		*outputDirPath = filepath.Base(targetDirPath)
	}

	if !*respectFileCase {
		*fileExtsCsv = strings.ToLower(*fileExtsCsv)
	}
	fileExts := strings.Split(*fileExtsCsv, ",")

	decompilerPool, err := grp.NewParallelFuncPool(*numDecompilers)
	if err != nil {
		log.Fatalf("failed to create parallel function pool - %s", err.Error())
	}
	onJobComplete := make(chan struct{})
	numFiles := 0

	fileWalkerConfig := filesearch.FindUniqueFilesConfig{
		TargetDirPath: targetDirPath,
		Recursive:     *scanRecursively,
		AllowDupes:    *allowDuplicateFiles,
		IncludeFileFn: func(fullFilePath string) (shouldInclude bool) {
			if !*respectFileCase {
				fullFilePath = strings.ToLower(fullFilePath)
			}
			for i := range fileExts {
				if strings.HasSuffix(fullFilePath, fileExts[i]) {
					return true
				}
			}
			return false
		},
		FoundFileFn: func(info filesearch.StatefulFileInfo) error {
			finalOutputDirPath := finalOutputDirCalc{
				searchAbsPath:     info.AbsSearchDirPath,
				targetFileAbsPath: info.FilePath,
				outputDirPath:     *outputDirPath,
			}.get()

			if info.AlreadySeen {
				err = os.MkdirAll(finalOutputDirPath, 0700)
				if err != nil {
					return fmt.Errorf("failed to create directory for ignored .NET file '%s' - %s",
						info.FilePath, err.Error())
				}
				err = ioutil.WriteFile(filepath.Join(finalOutputDirPath, "ignored.log"),
					[]byte(fmt.Sprintf("file has already been seen at '%s', hash of file is %s\n",
						info.FilePath, info.Hash)),
					0600)
				if err != nil {
					return fmt.Errorf("failed to create log for ignored .NET file '%s' - %s",
						info.FilePath, err.Error())
				}
				return nil
			}

			numFiles++

			decompilerPool.QueueFunction(func() error {
				defer func() {
					onJobComplete <- struct{}{}
				}()

				err := decompileNETFile(decompileNETInfo{
					ilspycmdPath:       *ilspycmdPath,
					filePath:           info.FilePath,
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
						if *verbose {
							log.Printf("[warn] %s", err.Error())
						}
						return nil
					}
					return fmt.Errorf("failed to decompile '%s' - %s",
						info.FilePath, err.Error())
				}

				err = ioutil.WriteFile(filepath.Join(finalOutputDirPath, "hash.txt"),
					[]byte(fmt.Sprintf("%s\n", info.Hash)),
					0600)
				if err != nil {
					return fmt.Errorf("failed to write hash file for '%s' - %s",
						info.FilePath, info.Hash)
				}

				if *verbose {
					log.Printf("decompiled '%s' to '%s'",
						info.FilePath, finalOutputDirPath)
				}

				return nil
			})

			return nil
		},
	}

	// Display some messages if it is taking a while.
	searchFinished := make(chan struct{})
	go func() {
		timeout := 5*time.Second
		timer := time.NewTimer(timeout)
		for {
			select {
			case <-timer.C:
				log.Println("still searching for files to decompile, sorry for the wait :(")
				timeout = timeout * 3
				timer.Reset(timeout)
			case <-searchFinished:
				timer.Stop()
				select {
				case <-timer.C:
				default:
				}
				return
			}
		}
	}()

	start := time.Now()
	err = filesearch.FindUniqueFiles(fileWalkerConfig)
	close(searchFinished)
	if err != nil {
		log.Fatal(err.Error())
	}

	var bar *progressbar.ProgressBar
	if !*verbose {
		bar = progressbar.NewOptions(numFiles, progressbar.OptionShowCount())
	}
	go func() {
		for range onJobComplete {
			if bar != nil {
				bar.Add(1)
			}
		}
	}()

	err = decompilerPool.Wait()
	close(onJobComplete)
	if err != nil {
		log.Fatalln(err.Error())
	}

	if bar != nil {
		bar.Finish()
	}

	if *verbose {
		log.Printf("finished after %s", time.Since(start).String())
	}
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

	raw, err := exec.Command(info.ilspycmdPath, info.filePath, "-p", "-o", info.finalOutputDirPath).CombinedOutput()
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
