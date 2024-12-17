package common

import (
	"container/heap"
	"context"
	"go.uber.org/zap"
	"sync"
	"time"
)

// Task 定时任务接口
type Task interface {
	ID() string                    // 任务ID
	NextTime() time.Time           // 下次执行时间
	Execute(context.Context) error // 执行任务
	IsRepeat() bool                // 是否重复执行
	GetInterval() time.Duration    // 重复间隔
}

// TaskHeap 任务优先级队列
// 使用切片实现的堆，不需要使用指针接收者
type TaskHeap []Task

func (h *TaskHeap) Len() int {
	return len(*h)
}

func (h *TaskHeap) Less(i, j int) bool {
	return (*h)[i].NextTime().Before((*h)[j].NextTime())
}

func (h *TaskHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
}

func (h *TaskHeap) Push(x interface{}) {
	*h = append(*h, x.(Task))
}

func (h *TaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Scheduler 调度器
type Scheduler struct {
	tasks   TaskHeap        // 任务优先级队列
	taskMap map[string]Task // 任务映射表
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:   make(TaskHeap, 0),
		taskMap: make(map[string]Task),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// AddTask 添加任务
func (s *Scheduler) AddTask(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.taskMap[task.ID()]; exists {
		return
	}

	s.taskMap[task.ID()] = task
	heap.Push(&s.tasks, task)
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.taskMap, taskID)
	// 重建堆
	newTasks := make(TaskHeap, 0)
	for _, task := range s.tasks {
		if task.ID() != taskID {
			newTasks = append(newTasks, task)
		}
	}
	s.tasks = newTasks
	heap.Init(&s.tasks)
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	GetLogger().Info("Scheduler starting...")
	go s.run()
	GetLogger().Info("Scheduler started successfully")
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	GetLogger().Info("Scheduler stopping...")
	s.cancel()
	GetLogger().Info("Scheduler stopped successfully")
	return nil
}

// run 运行调度循环
func (s *Scheduler) run() {
	GetLogger().Info("Scheduler run loop started")

	defer func() {
		if r := recover(); r != nil {
			GetLogger().Error("Scheduler panic recovered",
				zap.Any("error", r))
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			GetLogger().Info("Scheduler context cancelled, stopping run loop")
			return
		default:
			s.mu.Lock()
			if len(s.tasks) == 0 {
				s.mu.Unlock()
				time.Sleep(time.Second)
				continue
			}

			task := heap.Pop(&s.tasks).(Task)
			now := time.Now()
			nextTime := task.NextTime()

			if nextTime.After(now) {
				heap.Push(&s.tasks, task)
				s.mu.Unlock()
				time.Sleep(time.Second)
				continue
			}

			GetLogger().Info("Executing task",
				zap.String("task_id", task.ID()),
				zap.Time("scheduled_time", nextTime),
				zap.Time("actual_time", now),
				zap.Bool("is_repeat", task.IsRepeat()))

			// 执行任务
			go func(t Task) {
				startTime := time.Now()
				if err := t.Execute(s.ctx); err != nil {
					GetLogger().Error("Task execution failed",
						zap.String("task_id", t.ID()),
						zap.Error(err),
						zap.Duration("duration", time.Since(startTime)))
				} else {
					GetLogger().Info("Task executed successfully",
						zap.String("task_id", t.ID()),
						zap.Duration("duration", time.Since(startTime)))
				}

				s.mu.Lock()
				defer s.mu.Unlock()

				if t.IsRepeat() {
					nextTask := &BaseTask{
						id:       t.ID(),
						nextTime: time.Now().Add(t.GetInterval()),
						execute:  t.Execute,
						repeat:   true,
						interval: t.GetInterval(),
					}
					heap.Push(&s.tasks, nextTask)
					GetLogger().Info("Repeat task rescheduled",
						zap.String("task_id", t.ID()),
						zap.Time("next_time", nextTask.nextTime))
				} else {
					delete(s.taskMap, t.ID())
					GetLogger().Info("One-time task removed",
						zap.String("task_id", t.ID()))
				}
			}(task)

			s.mu.Unlock()
		}
	}
}

// BaseTask 基础任务实现
type BaseTask struct {
	id       string
	nextTime time.Time
	execute  func(context.Context) error
	repeat   bool
	interval time.Duration
}

func (t *BaseTask) ID() string                        { return t.id }
func (t *BaseTask) NextTime() time.Time               { return t.nextTime }
func (t *BaseTask) Execute(ctx context.Context) error { return t.execute(ctx) }
func (t *BaseTask) IsRepeat() bool                    { return t.repeat }
func (t *BaseTask) GetInterval() time.Duration        { return t.interval }

// NewTask 创建一次性任务
func NewTask(id string, executeTime time.Time, execute func(context.Context) error) Task {
	return &BaseTask{
		id:       id,
		nextTime: executeTime,
		execute:  execute,
		repeat:   false,
	}
}

// NewRepeatTask 创建重复任务
func NewRepeatTask(id string, interval time.Duration,
	execute func(context.Context) error) Task {
	return &BaseTask{
		id:       id,
		nextTime: time.Now().Add(interval),
		execute:  execute,
		repeat:   true,
		interval: interval,
	}
}

// GameTask 游戏任务示例
type GameTask struct {
	BaseTask
	playerID string
	gameData interface{}
}

// NewDailyResetTask 每日重置任务
func NewDailyResetTask(playerID string) Task {
	return &GameTask{
		BaseTask: BaseTask{
			id:       "daily_reset_" + playerID,
			nextTime: getNextDayResetTime(),
			repeat:   true,
			interval: 24 * time.Hour,
		},
		playerID: playerID,
	}
}

func getNextDayResetTime() time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// NewWeeklyTask 每周任务
func NewWeeklyTask(playerID string) Task {
	return &GameTask{
		BaseTask: BaseTask{
			id:       "weekly_task_" + playerID,
			nextTime: getNextWeekResetTime(),
			repeat:   true,
			interval: 7 * 24 * time.Hour,
		},
		playerID: playerID,
	}
}

func getNextWeekResetTime() time.Time {
	now := time.Now()
	weekday := time.Monday
	days := int(weekday - now.Weekday())
	if days <= 0 {
		days += 7
	}
	next := time.Date(now.Year(), now.Month(),
		now.Day(), 0, 0, 0, 0, now.Location())
	next = next.AddDate(0, 0, days)
	return next
}
