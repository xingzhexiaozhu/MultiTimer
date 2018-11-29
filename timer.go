package Exp

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

// 任务数据结构
type Task struct {
	Tid        string // 任务标识
	LastRunTime int64  // 任务最后一次执行时间
	Interval    int64  // 任务执行的时间间隔
	Job         Job    // 要执行的任务
}

// 任务执行的具体接口，具体执行什么由传入函数决定
type Job interface {
	Run()
}

type FuncJob func()

func (f FuncJob) Run() {
	f()
}

// 按指定条件生成新任务
func newTask(lastTime, intervel int64, f func()) *Task {
	return &Task{
		Tid:        uuid.New().String(),
		LastRunTime: lastTime,
		Interval:    intervel,
		Job:         FuncJob(f),
	}
}

// 任务调度器数据结构
type TaskScheduler struct {
	tasks  []*Task       // 任务队列，存放所有任务
	swap   []*Task       // 任务缓冲队列，新加入任务时如果任务队列处于加锁状态则先加入该队列
	add    chan *Task    // 向任务队列中添加一个元素
	remove chan string   // 从任务队列中移除一个元素
	lock   bool          // 任务队列加锁状态
	stop   chan struct{} // 停止任务调度器
	//runnig bool          // 任务调度器运行状态
}

func NewScheduler() *TaskScheduler {
	return &TaskScheduler{
		tasks:  make([]*Task, 0),
		swap:   make([]*Task, 0),
		add:    make(chan *Task),
		remove: make(chan string),
		stop:   make(chan struct{}),
		lock:   false,
		//runnig: false,
	}
}

type Lock interface {
	Lock()
	UnLock()
}

// 任务调度器加锁
func (scheduler *TaskScheduler) Lock() {
	scheduler.lock = true
}

// 任务调度器解锁
func (scheduler *TaskScheduler) UnLock() {
	scheduler.lock = false
	if len(scheduler.swap) > 0 {
		for _, task := range scheduler.swap {
			// 任务调度器解锁时将任务缓冲队列中的任务全部加入任务队列
			scheduler.tasks = append(scheduler.tasks, task)
		}
		scheduler.swap = make([]*Task, 0)
	}
}

// 添加新任务
func (scheduler *TaskScheduler) AddTask(task *Task) string {
	// 新任务生成任务id
	if task.Tid == "" {
		task.Tid = uuid.New().String()
	}

	if scheduler.lock {
		// 如果任务队列处于加锁状态，则先将任务加入任务缓冲队列
		fmt.Println("locked")
		scheduler.swap = append(scheduler.swap, task)
		scheduler.add <- task
	} else {
		scheduler.tasks = append(scheduler.tasks, task)
		scheduler.add <- task
	}
	return task.Tid
}

// 查看任务调度器中所有任务
func (scheduler *TaskScheduler) ScanTask() []*Task {
	return scheduler.tasks
}

// 按任务id删除任务
func (scheduler *TaskScheduler) RemoveTask(tid string) {
	scheduler.Lock()
	defer scheduler.UnLock()
	for k, t := range scheduler.tasks {
		if t.Tid == tid {
			scheduler.tasks = append(scheduler.tasks[:k], scheduler.tasks[:k+1]...)
			break
		}
	}
}

// 启动任务调度器
func (scheduler *TaskScheduler) Start() {
	//if scheduler.runnig {
	//	fmt.Println("start")
	//	return
	//}
	//scheduler.runnig = true

	task := newTask(time.Now().Unix(), 24*60*60, func() {
		fmt.Println("init")
	})
	scheduler.tasks = append(scheduler.tasks, task)

	go scheduler.run()
}

// 停止一个任务
func (scheduler *TaskScheduler) StopOne(tid string) {
	scheduler.remove <- tid
}

// 停止所有任务
func (scheduler *TaskScheduler) Stop() {
	//if !scheduler.runnig {
	//	fmt.Println("stop")
	//	return
	//}
	scheduler.stop <- struct{}{}
	//scheduler.runnig = false
}

// 任务调度器执行过程
func (scheduler *TaskScheduler) run() {
	for {
		now := time.Now()
		task, idx := scheduler.GetTask()
		timeSpan := task.LastRunTime - now.Unix()

		if timeSpan <= 0 {
			scheduler.tasks[idx].LastRunTime = now.Unix()
			if task != nil {
				go task.Job.Run()
			}
			scheduler.doAndReset(idx) // 将执行的任务加入任务队列的队尾
			continue
		}
		sec := task.LastRunTime / int64(time.Second)
		nsec := task.LastRunTime % int64(time.Second)
		span := time.Unix(sec, nsec).Sub(now)
		timer := time.NewTicker(span)
		for {
			select {
			case <-timer.C:
				scheduler.doAndReset(idx)
				if task != nil {
					go task.Job.Run()
					timer.Stop()
				}
			case <-scheduler.add:
				timer.Stop()
			case tid := <-scheduler.remove:
				scheduler.RemoveTask(tid)
				timer.Stop()
			case <-scheduler.stop:
				timer.Stop()
				return
			}
			break
		}
	}
}

// 获取应该执行的任务
func (scheduler *TaskScheduler) GetTask() (task *Task, index int) {
	scheduler.Lock()
	defer scheduler.UnLock()

	min := scheduler.tasks[0].LastRunTime
	index = 0

	for idx, ltask := range scheduler.tasks {
		if min <= ltask.LastRunTime {
			continue
		} else {
			index = idx
			min = ltask.LastRunTime
			continue
		}
	}

	task = scheduler.tasks[index]
	return task, index
}

// 将执行的任务再加入任务队列
func (scheduler *TaskScheduler) doAndReset(idx int) {
	scheduler.Lock()
	defer scheduler.UnLock()

	if idx < len(scheduler.tasks) {
		nowTask := scheduler.tasks[idx]
		scheduler.tasks = append(scheduler.tasks[:idx], scheduler.tasks[idx+1:]...)

		scheduler.tasks = append(scheduler.tasks, nowTask)
	}
}
