package ui

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/gtk"
	"github.com/mcuadros/go-octoprint"
)

var statusPanelInstance *statusPanel

type statusPanel struct {
	CommonPanel
	step *StepButton
	pb   *gtk.ProgressBar

	bed, tool0         *LabelWithImage
	file, left         *LabelWithImage
	print, pause, stop *gtk.Button
}

func StatusPanel(ui *UI, parent Panel) Panel {
	if statusPanelInstance == nil {
		m := &statusPanel{CommonPanel: NewCommonPanel(ui, parent)}
		m.b = NewBackgroundTask(time.Second*5, m.update)
		m.initialize()

		statusPanelInstance = m
	}

	return statusPanelInstance
}

func (m *statusPanel) initialize() {
	defer m.Initialize()

	m.Grid().Attach(m.createMainBox(), 1, 0, 4, 2)
}

func (m *statusPanel) createProgressBar() *gtk.ProgressBar {
	m.pb = MustProgressBar()
	m.pb.SetShowText(true)
	m.pb.SetMarginTop(12)
	m.pb.SetMarginStart(5)
	m.pb.SetMarginEnd(5)
	m.pb.SetMarginBottom(17)

	return m.pb
}

func (m *statusPanel) createMainBox() *gtk.Box {
	grid := MustGrid()
	grid.SetHExpand(true)
	grid.Add(m.createInfoBox())
	grid.SetVAlign(gtk.ALIGN_START)
	grid.SetMarginTop(20)

	butt := MustBox(gtk.ORIENTATION_HORIZONTAL, 5)
	butt.SetHAlign(gtk.ALIGN_END)
	butt.SetVAlign(gtk.ALIGN_END)
	butt.SetVExpand(true)
	butt.SetMarginTop(5)
	butt.SetMarginEnd(5)
    butt.Add(m.createPrintButton())
    butt.Add(m.createPauseButton())
    butt.Add(m.createStopButton())
	butt.Add(MustButton(MustImageFromFile("back.svg"), m.UI.GoHistory))

	box := MustBox(gtk.ORIENTATION_VERTICAL, 5)
	box.SetVAlign(gtk.ALIGN_START)
	box.SetVExpand(true)
	box.Add(grid)
	box.Add(m.createProgressBar())
	box.Add(butt)
	return box
}

func (m *statusPanel) createInfoBox() *gtk.Box {
	m.file = MustLabelWithImage("file.svg", "")
	m.left = MustLabelWithImage("speed-step.svg", "")
	m.bed = MustLabelWithImage("bed.svg", "")
	m.tool0 = MustLabelWithImage("extruder.svg", "")

	info := MustBox(gtk.ORIENTATION_VERTICAL, 5)
	info.SetHAlign(gtk.ALIGN_START)
	info.SetHExpand(true)
	info.Add(m.file)
	info.Add(m.left)
	info.Add(m.tool0)
	info.Add(m.bed)
	info.SetMarginStart(20)

	return info
}

func (m *statusPanel) createPrintButton() gtk.IWidget {
	m.print = MustButton(MustImageFromFile("status.svg"), func() {
		defer m.updateTemperature()

		Logger.Warning("Starting a new job")
		if err := (&octoprint.StartRequest{}).Do(m.UI.Printer); err != nil {
			Logger.Error(err)
			return
		}
	})

	return m.print
}

func (m *statusPanel) createPauseButton() gtk.IWidget {
	m.pause = MustButton(MustImageFromFile("pause.svg"), func() {
		defer m.updateTemperature()

		Logger.Warning("Pausing/Resuming job")
		cmd := &octoprint.PauseRequest{Action: octoprint.Toggle}
		if err := cmd.Do(m.UI.Printer); err != nil {
			Logger.Error(err)
			return
		}
	})

	return m.pause
}

func (m *statusPanel) createStopButton() gtk.IWidget {
	m.stop = MustButton(MustImageFromFile("stop.svg"), func() {
		defer m.updateTemperature()

		Logger.Warning("Stopping job")
		if err := (&octoprint.CancelRequest{}).Do(m.UI.Printer); err != nil {
			Logger.Error(err)
			return
		}
	})

	return m.stop
}

func (m *statusPanel) update() {
	m.updateTemperature()
	m.updateJob()
}

func (m *statusPanel) updateTemperature() {
	s, err := (&octoprint.StateRequest{Exclude: []string{"sd"}}).Do(m.UI.Printer)
	if err != nil {
		Logger.Error(err)
		return
	}

	m.doUpdateState(&s.State)

	for tool, s := range s.Temperature.Current {
		text := fmt.Sprintf("%.0f°C ⇒ %.0f°C ", s.Actual, s.Target)
		switch tool {
		case "bed":
			m.bed.Label.SetLabel(text)
		case "tool0":
			m.tool0.Label.SetLabel(text)
		}
	}
}

func (m *statusPanel) doUpdateState(s *octoprint.PrinterState) {
	switch {
	case s.Flags.Printing:
		m.print.SetSensitive(false)
		m.pause.SetSensitive(true)
		m.stop.SetSensitive(true)
	case s.Flags.Paused:
		m.print.SetSensitive(false)
		m.pause.SetImage(MustImageFromFileWithSize("resume.svg", 40, 40))
		m.pause.SetSensitive(true)
		m.stop.SetSensitive(true)
		return
	case s.Flags.Ready:
		m.print.SetSensitive(true)
		m.pause.SetSensitive(false)
		m.stop.SetSensitive(false)
	default:
		m.print.SetSensitive(false)
		m.pause.SetSensitive(false)
		m.stop.SetSensitive(false)
	}

	m.pause.SetImage(MustImageFromFileWithSize("pause.svg", 40, 40))
}

func (m *statusPanel) updateJob() {
	s, err := (&octoprint.JobRequest{}).Do(m.UI.Printer)
	if err != nil {
		Logger.Error(err)
		return
	}

	file := "<i>File not set</i>"
	if s.Job.File.Name != "" {
		file = filenameEllipsis_long(s.Job.File.Name)
	}

	m.file.Label.SetLabel(fmt.Sprintf("%s", file))
	m.pb.SetFraction(s.Progress.Completion / 100)

	if m.UI.State.IsOperational() {
		m.left.Label.SetLabel("Printer is ready")
		return
	}

	var text string
	switch s.Progress.Completion {
	case 100:
		text = fmt.Sprintf("Completed in %s", time.Duration(int64(s.Job.LastPrintTime)*1e9))
	case 0:
		text = "Warming up ..."
	default:
		e := time.Duration(int64(s.Progress.PrintTime) * 1e9)
		l := time.Duration(int64(s.Progress.PrintTimeLeft) * 1e9)
		text = fmt.Sprintf("Elapsed: %s / Left: %s", e, l)
		if l == 0 {
			text = fmt.Sprintf("Elapsed: %s", e)
		}
	}

	m.left.Label.SetLabel(text)
}

func filenameEllipsis_long(name string) string {
	if len(name) > 35 {
		return name[:32] + "…"
	}

	return name
}

func filenameEllipsis(name string) string {
	if len(name) > 31 {
		return name[:28] + "…"
	}

	return name
}


func filenameEllipsis_short(name string) string {
	if len(name) > 27 {
		return name[:24] + "…"
	}

	return name
}
