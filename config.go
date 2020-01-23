package main

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type config struct {
	GProject  int      `yaml:"GProject"`
	GPUrl     string   `yaml:"GPUrl"`
	GToken    string   `yaml:"GToken"`
	GURL      string   `yaml:"GURL"`
	GUser     string   `yaml:"GUser"`
	LBase     string   `yaml:"LBase"`
	LHost     string   `yaml:"LHost"`
	LPass     string   `yaml:"LPass"`
	LUser     string   `yaml:"LUser"`
	MBad      string   `yaml:"MBad"`
	MDown     string   `yaml:"MDown"`
	MFail     string   `yaml:"MFail"`
	MGood     string   `yaml:"MGood"`
	MUp       string   `yaml:"MUp"`
	SHost     string   `yaml:"SHost"`
	SMail     string   `yaml:"SMail"`
	SPass     string   `yaml:"SPass"`
	SPort     int      `yaml:"SPort"`
	SUser     string   `yaml:"SUser"`
	VBackend  []string `yaml:"VBackend"`
	VFrontend []string `yaml:"VFrontend"`
}

func (c *config) getConfig() *config {

	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}
