package pool

import (
	"go-scheduler/internal/domain"
	errs "go-scheduler/internal/error"
)

func (p *Pool) AddJob(job Job) error {
	meta := job.GetMetadata()
	if _, ok := p.Jobs.Load(meta.ID); ok {
		return errs.New(errs.ErrIDExists, meta.ID)
	}
	if job.GetStatus() != domain.Waiting {
		return errs.New(errs.ErrJobWrongStatus, meta.ID)
	}
	p.Jobs.Store(meta.ID, job)
	return nil
}

func (p *Pool) RemoveJob(id string) error {
	job, err := p.GetJobByID(id)
	if err != nil {
		return err
	}
	job.Stop()
	p.Jobs.Delete(id)
	return nil
}

func (p *Pool) PauseJob(id string) error {
	job, err := p.GetJobByID(id)
	if err != nil {
		return err
	}
	job.Pause()
	return nil
}

func (p *Pool) ResumeJob(id string) error {
	job, err := p.GetJobByID(id)
	if err != nil {
		return err
	}
	job.Resume()
	return nil
}
