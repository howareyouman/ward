package app

import (
	"bytes"
	"fmt"
	"github.com/Kami-no/ward/src/app/client"
	"github.com/Kami-no/ward/src/app/ldap"
	"github.com/Kami-no/ward/src/config"
	"github.com/xanzy/go-gitlab"
	"html/template"
	"log"
)

type MrAction struct {
	Pid      int
	Mid      int
	Aid      int
	Award    string
	MergedBy string
	Path     string
	State    bool
}

func deadAuthorTemplate(dAuthor client.DeadAuthor) (string, error) {
	var buffer bytes.Buffer
	var output string

	tmpl := template.Must(template.ParseFiles("templates/dead-branches-author.gohtml"))
	err := tmpl.Execute(&buffer, dAuthor)
	if err != nil {
		return output, err
	}
	output = buffer.String()

	return output, nil
}

func DetectMR(ldapService ldap.Service, client client.GitlabClient, cfg *config.Config) []MrAction {
	mrsOpened, err := client.CheckPrjRequests(cfg.Projects, "opened")
	if err != nil {
		log.Println(err)
	}
	actionsOpened := evalOpenedRequests(mrsOpened)

	mrsMerged, err := client.CheckPrjRequests(cfg.Projects, "merged")
	if err != nil {
		log.Println(err)
	}
	actionsMerged := evalMergedRequests(mrsMerged)

	actions := append(actionsOpened, actionsMerged...)

	processMR(ldapService, cfg, actions)

	return actions
}

func evalOpenedRequests(MRProjects map[int]client.MrProject) []MrAction {
	var actions []MrAction

	for pid, project := range MRProjects {
		for mid, mr := range project.MR {
			if mr.Awards.Like && !mr.Awards.Dislike {
				if mr.Awards.NotReady != 0 {
					action := MrAction{
						Pid:   pid,
						Mid:   mid,
						Aid:   mr.Awards.NotReady,
						Award: "notready",
						State: false}
					actions = append(actions, action)
				}
				if mr.Awards.Ready == 0 {
					action := MrAction{
						Pid:   pid,
						Mid:   mid,
						Aid:   mr.Awards.Ready,
						Award: "ready",
						State: true}
					actions = append(actions, action)
				}
			} else {
				if mr.Awards.Ready != 0 {
					action := MrAction{
						Pid:   pid,
						Mid:   mid,
						Aid:   mr.Awards.Ready,
						Award: "ready",
						State: false}
					actions = append(actions, action)
				}
				if mr.Awards.NotReady == 0 {
					action := MrAction{
						Pid:      pid,
						Mid:      mid,
						Aid:      mr.Awards.NotReady,
						Award:    "notready",
						MergedBy: mr.MergedBy,
						State:    true}
					actions = append(actions, action)
				}
			}

			if mr.Awards.NonCompliant != 0 {
				action := MrAction{
					Pid:   pid,
					Mid:   mid,
					Aid:   mr.Awards.NonCompliant,
					Award: "nc",
					State: false}
				actions = append(actions, action)
			}
		}
	}

	return actions
}

func evalMergedRequests(MRProjects map[int]client.MrProject) []MrAction {
	var actions []MrAction

	for pid, project := range MRProjects {
		for mid, mr := range project.MR {
			if mr.Awards.Dislike || !mr.Awards.Like {
				if mr.Awards.NonCompliant == 0 {
					action := MrAction{
						Pid:      pid,
						Mid:      mid,
						Aid:      mr.Awards.NonCompliant,
						Award:    "nc",
						MergedBy: mr.MergedBy,
						Path:     mr.Path,
						State:    true}
					actions = append(actions, action)
				}
			} else {
				if mr.Awards.NonCompliant != 0 {
					action := MrAction{
						Pid:   pid,
						Mid:   mid,
						Aid:   mr.Awards.NonCompliant,
						Award: "nc",
						State: false}
					actions = append(actions, action)
				}
			}

			if mr.Awards.NotReady != 0 {
				action := MrAction{
					Pid:   pid,
					Mid:   mid,
					Aid:   mr.Awards.NotReady,
					Award: "notready",
					State: false}
				actions = append(actions, action)
			}

			if mr.Awards.Ready != 0 {
				action := MrAction{
					Pid:   pid,
					Mid:   mid,
					Aid:   mr.Awards.Ready,
					Award: "ready",
					State: false}
				actions = append(actions, action)
			}
		}
	}

	return actions
}

func processMR(ldapService ldap.Service, cfg *config.Config, actions []MrAction) {
	award := map[string]string{
		"ready":    cfg.Awards.Ready,
		"notready": cfg.Awards.NotReady,
		"nc":       cfg.Awards.NonCompliant,
	}

	gitOpts := gitlab.WithBaseURL(cfg.Endpoints.GitLab)
	git, err := gitlab.NewBasicAuthClient(
		cfg.Credentials.User, cfg.Credentials.Password, gitOpts)
	if err != nil {
		fmt.Printf("Failed to connect to GitLab: %v", err)
	}

	for _, action := range actions {
		if action.State {
			awardOpts := &gitlab.CreateAwardEmojiOptions{Name: award[action.Award]}
			_, _, _ = git.AwardEmoji.CreateMergeRequestAwardEmoji(action.Pid, action.Mid, awardOpts)

			// Notify reviewers (most likely onece per MR)
			if action.Award == "notready" {
				err := notifyReviewers(git, cfg.Projects[action.Pid].Teams, action.Pid, action.Mid)
				if err != nil {
					log.Printf("Failed to post notification message for %v@%v: %v",
						action.Mid, action.Pid, err)
				}
			}

			// Notify about non-compiant merge
			if action.Award == "nc" {
				var prjName string
				var prjUrl string
				var users []string
				var emails []string
				var subj string
				var msg string
				var ownersEmail []string
				var ownersUsers []string

				prjOpts := &gitlab.GetProjectOptions{}
				prj, _, err := git.Projects.GetProject(action.Pid, prjOpts)
				if err != nil {
					prjName = fmt.Sprintf("%v", action.Pid)
					prjUrl = cfg.Endpoints.GitLab
					log.Printf("Failed to get project info: %v", err)
				} else {
					prjName = prj.NameWithNamespace
					prjUrl = prj.WebURL
				}

				log.Printf("Non-compliant MR detected: %v@%v", action.Mid, action.Pid)

				users = append(users, action.MergedBy)
				emails = ldapService.ListMails(users)
				subj = "Code of Conduct failure incident"
				msg = fmt.Sprintf(
					"Hello,"+
						"<p>By merging <a href='%v'>Merge Request #%v</a> in project "+
						"<a href='%v'>%v</a> without 2 qualified approves "+
						"or negative review you've failed repository's Code of Conduct.</p>"+
						"<p>This incident will be reported.</p>",
					action.Path, action.Mid, prjUrl, prjName)

				if err := mailSend(cfg, emails, subj, msg); err != nil {
					log.Printf("Failed to send mail to recipient: %v", err)
				}

				for _, team := range cfg.Projects[action.Pid].Teams {
					ownersUsers = append(ownersUsers, team...)
				}

				ownersEmail = ldapService.ListMails(ownersUsers)

				subj = fmt.Sprintf("MR %v has failed requirements!", action.Mid)
				msg = fmt.Sprintf(
					"<p><a href='%v'>Merge Request #%v</a> in project <a href='%v'>%v</a> "+
						"does not meet requirements but it was merged!</p>",
					action.Path, action.Mid, prjUrl, prjName)

				if err := mailSend(cfg, ownersEmail, subj, msg); err != nil {
					log.Printf("Failed to send mail to owners: %v", err)
				}
			}
		} else {
			_, _ = git.AwardEmoji.DeleteMergeRequestAwardEmoji(action.Pid, action.Mid, action.Aid)
		}
	}
}

func notifyReviewers(git *gitlab.Client, reviewers map[string][]string, pid int, mid int) error {
	msg := "Notifying reviewers:"
	for _, team := range reviewers {
		for _, user := range team {
			msg = fmt.Sprintf("%v @%v", msg, user)
		}
	}

	noteOpts := gitlab.CreateMergeRequestNoteOptions{
		Body: &msg,
	}
	_, _, err := git.Notes.CreateMergeRequestNote(pid, mid, &noteOpts)

	return err
}
