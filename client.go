package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

func client() (chan struct{}, error) {
	type templateArguments struct {
		Devicename  string
		UdsPathName string
	}
	type generatedFile struct {
		templateFilename string
		templateArgs     templateArguments
		compileAble      bool
	}
	srcfiles := map[string]generatedFile{
		"config.h": generatedFile{
			templateFilename: "templates/config_h.gotmpl",
			compileAble:      false,
		},
		"serial-channel.h": generatedFile{
			templateFilename: "templates/serial-channel_h.gotmpl",
			compileAble:      false,
		},
		"uds-channel.h": generatedFile{
			templateFilename: "templates/uds-channel_h.gotmpl",
			compileAble:      false,
		},
		"serial-channel.c": generatedFile{
			templateFilename: "templates/serial-channel.gotmpl",
			templateArgs: templateArguments{
				Devicename:  device,
				UdsPathName: uds_file_path,
			},
			compileAble: true,
		},
		"uds-channel.c": generatedFile{
			templateFilename: "templates/uds_channel_c.gotmpl",
			templateArgs: templateArguments{
				Devicename:  device,
				UdsPathName: uds_file_path,
			},
			compileAble: true,
		},
		"serial.c": generatedFile{
			templateFilename: "templates/serial_c.gotmpl",
			templateArgs: templateArguments{
				Devicename:  device,
				UdsPathName: uds_file_path,
			},
			compileAble: true,
		},
	}

	compiler := "gcc"
	obj_files := []string{}
	for _, genfile := range []string{
		"config.h",
		"serial-channel.h",
		"uds-channel.h",
		"serial-channel.c",
		"uds-channel.c",
		"serial.c",
	} {
		k := srcfiles[genfile]
		tpl := template.Must(template.ParseFiles(k.templateFilename))
		fn := genfile
		f, err := os.Create(fn)
		if err != nil {
			return nil, err
		}
		err = tpl.Execute(f, k.templateArgs)
		if err != nil {
			return nil, err
		}
		f.Close()

		if k.compileAble {
			args := []string{"-I.", "-c", "-ggdb3", "-Wall", "-Werror", genfile}
			cmd := exec.Command(compiler, args...)
			err := cmd.Run()
			if err != nil {
				return nil, errors.New(fmt.Sprintf("error while executing %v: %v", cmd, err))
			}
			obj_files = append(obj_files, strings.ReplaceAll(genfile, ".c", ".o"))
		}
	}
	args := []string{"-o", "serial"}
	args = append(args, obj_files...)
	cmd := exec.Command(compiler, args...)
	err := cmd.Run()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error while executing %v: %v", cmd, err))
	}

	// remove generated files
	for _, genfile := range []string{
		"config.h",
		"serial-channel.h",
		"uds-channel.h",
		"serial-channel.c",
		"uds-channel.c",
		"serial.c",
	} {
		os.Remove(genfile)
		if strings.HasSuffix(genfile, ".c") {
			objf := genfile[:len(genfile)-2] + ".o"
			os.Remove(objf)
		}
	}

	cmd = exec.Command("./serial")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error: could not take stderr from child command: %v\n", err))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error: could not take stdout from child command: %v\n", err))
	}
	stdout_copied := make(chan struct{})
	stderr_copied := make(chan struct{})
	go func() {
		io.Copy(os.Stdout, stdout)
		stdout_copied <- struct{}{}
	}()
	go func() {
		io.Copy(os.Stderr, stderr)
		stderr_copied <- struct{}{}
	}()
	err = cmd.Start()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error while executing %v: %v", cmd, err))
	}

	done := make(chan struct{})
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Fatalf("child process ./serial finished with error: %v\n", err)
		}
		<-stdout_copied
		<-stderr_copied
		done <- struct{}{}
	}()
	return done, nil
}
