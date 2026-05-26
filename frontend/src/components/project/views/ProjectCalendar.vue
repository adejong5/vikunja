<template>
	<ProjectWrapper
		class="project-calendar"
		:is-loading-project="isLoadingProject"
		:project-id="projectId"
		:view-id
	>
		<template #default>
			<div class="calendar-container">
				<div class="calendar-nav">
					<XButton
						variant="secondary"
						icon="angle-left"
						@click="prevMonth"
					>
						{{ $t('misc.previous') }}
					</XButton>
					<span class="calendar-nav__title">
						{{ monthTitle }}
					</span>
					<XButton
						variant="secondary"
						icon="angle-right"
						@click="nextMonth"
					>
						{{ $t('misc.next') }}
					</XButton>
					<XButton
						variant="secondary"
						class="mls-2"
						@click="goToToday"
					>
						{{ $t('project.calendar.today') }}
					</XButton>
				</div>

				<div class="calendar-grid">
					<div
						v-for="d in 7"
						:key="d"
						class="calendar-grid__header"
					>
						{{ $t(`project.calendar.weekdays.${d - 1}`) }}
					</div>

					<div
						v-for="cell in calendarCells"
						:key="cell.key"
						class="calendar-grid__cell"
						:class="{
							'is-other-month': !cell.currentMonth,
							'is-today': cell.isToday,
							'is-selected': selectedDate && isSameDay(cell.date, selectedDate),
						}"
						@click="selectDay(cell.date)"
					>
						<span class="calendar-grid__day-number">{{ cell.date.getDate() }}</span>
						<div class="calendar-grid__tasks">
							<span
								v-for="task in tasksForDay(cell.date).slice(0, 3)"
								:key="task.id"
								class="calendar-grid__task-chip"
								:class="{'is-done': task.done}"
								:title="task.title"
								@click.stop="openTask(task)"
							>
								{{ task.title }}
							</span>
							<span
								v-if="tasksForDay(cell.date).length > 3"
								class="calendar-grid__task-chip is-overflow"
							>
								+{{ tasksForDay(cell.date).length - 3 }}
							</span>
						</div>
					</div>
				</div>

				<div
					v-if="selectedDate"
					class="calendar-day-panel"
				>
					<div class="calendar-day-panel__header">
						<strong>{{ formatDateLong(selectedDate) }}</strong>
						<XButton
							variant="secondary"
							icon="times"
							:aria-label="$t('misc.close')"
							@click="selectedDate = null"
						/>
					</div>

					<div
						v-if="canWrite"
						class="calendar-day-panel__add"
					>
						<input
							v-model="newTaskTitle"
							class="input"
							type="text"
							:placeholder="$t('project.list.addPlaceholder')"
							@keydown.enter="addTaskForDay"
						/>
						<XButton
							:loading="addingTask"
							:disabled="newTaskTitle === '' || undefined"
							icon="plus"
							@click="addTaskForDay"
						>
							{{ $t('project.list.add') }}
						</XButton>
					</div>

					<p
						v-if="tasksForDay(selectedDate).length === 0"
						class="has-text-centered mbs-4"
					>
						{{ $t('project.calendar.noTasks') }}
					</p>

					<ul class="calendar-day-panel__task-list">
						<li
							v-for="task in tasksForDay(selectedDate)"
							:key="task.id"
							class="calendar-day-panel__task"
						>
							<input
								type="checkbox"
								:checked="task.done"
								@change="toggleDone(task)"
							/>
							<span
								class="calendar-day-panel__task-title"
								:class="{'is-done': task.done}"
								@click="openTask(task)"
							>
								{{ task.title }}
							</span>
						</li>
					</ul>
				</div>

				<!-- No due date tasks -->
				<div
					v-if="noDueDateTasks.length > 0"
					class="no-due-date-section"
				>
					<h3 class="no-due-date-section__title">
						{{ $t('project.calendar.noDueDate') }}
					</h3>
					<SingleTaskInProject
						v-for="task in noDueDateTasks"
						:key="task.id"
						:the-task="task"
						:all-tasks="noDueDateTasks"
						:can-mark-as-done="canWrite"
						@task-updated="loadTasks()"
					/>
				</div>
			</div>
		</template>
	</ProjectWrapper>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useRouter} from 'vue-router'

import type {IProject} from '@/modelTypes/IProject'
import type {IProjectView} from '@/modelTypes/IProjectView'
import type {ITask} from '@/modelTypes/ITask'

import {useTaskList} from '@/composables/useTaskList'
import {useBaseStore} from '@/stores/base'
import {PERMISSIONS} from '@/constants/permissions'
import {formatDateLong} from '@/helpers/time/formatDate'

import TaskService from '@/services/task'
import TaskModel from '@/models/task'

import ProjectWrapper from '@/components/project/ProjectWrapper.vue'
import XButton from '@/components/input/Button.vue'
import SingleTaskInProject from '@/components/tasks/partials/SingleTaskInProject.vue'

const props = defineProps<{
	projectId: IProject['id'],
	viewId: IProjectView['id'],
	isLoadingProject: boolean,
}>()

const router = useRouter()
const baseStore = useBaseStore()
const canWrite = computed(() => baseStore.currentProject?.maxPermission > PERMISSIONS.READ)

const {tasks, loadTasks} = useTaskList(
	() => props.projectId,
	() => props.viewId,
	{due_date: 'asc'},
)

// — Month navigation —
const today = new Date()
const displayYear = ref(today.getFullYear())
const displayMonth = ref(today.getMonth())

function prevMonth() {
	if (displayMonth.value === 0) {
		displayMonth.value = 11
		displayYear.value--
	} else {
		displayMonth.value--
	}
}

function nextMonth() {
	if (displayMonth.value === 11) {
		displayMonth.value = 0
		displayYear.value++
	} else {
		displayMonth.value++
	}
}

function goToToday() {
	displayYear.value = today.getFullYear()
	displayMonth.value = today.getMonth()
}

const monthTitle = computed(() =>
	new Date(displayYear.value, displayMonth.value, 1).toLocaleDateString(undefined, {month: 'long', year: 'numeric'}),
)

// — Calendar grid —
interface CalendarCell {
	key: string
	date: Date
	currentMonth: boolean
	isToday: boolean
}

function isSameDay(a: Date, b: Date): boolean {
	return a.getFullYear() === b.getFullYear()
		&& a.getMonth() === b.getMonth()
		&& a.getDate() === b.getDate()
}

const calendarCells = computed<CalendarCell[]>(() => {
	const year = displayYear.value
	const month = displayMonth.value
	const firstDay = new Date(year, month, 1)
	const lastDay = new Date(year, month + 1, 0)

	const startOffset = firstDay.getDay()
	const cells: CalendarCell[] = []

	for (let i = startOffset - 1; i >= 0; i--) {
		const d = new Date(year, month, -i)
		cells.push({key: d.toISOString(), date: d, currentMonth: false, isToday: false})
	}

	for (let day = 1; day <= lastDay.getDate(); day++) {
		const d = new Date(year, month, day)
		cells.push({
			key: d.toISOString(),
			date: d,
			currentMonth: true,
			isToday: isSameDay(d, today),
		})
	}

	const remaining = 7 - (cells.length % 7)
	if (remaining < 7) {
		for (let i = 1; i <= remaining; i++) {
			const d = new Date(year, month + 1, i)
			cells.push({key: d.toISOString(), date: d, currentMonth: false, isToday: false})
		}
	}

	return cells
})

// — Task lookup by day —
function dayKey(d: Date): string {
	return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

const tasksByDay = computed(() => {
	const map = new Map<string, ITask[]>()
	for (const task of tasks.value) {
		if (!task.dueDate) continue
		const key = dayKey(task.dueDate as Date)
		if (!map.has(key)) map.set(key, [])
		map.get(key)!.push(task)
	}
	return map
})

function tasksForDay(date: Date): ITask[] {
	return tasksByDay.value.get(dayKey(date)) ?? []
}

// Strip subtasks-with-due-dates from relatedTasks recursively so
// SingleTaskInProject only nests tasks that also have no due date.
function filterSubtasks(task: ITask, noDueDateIds: Set<number>): ITask {
	const subtasks = task.relatedTasks?.subtask ?? []
	return {
		...task,
		relatedTasks: {
			...task.relatedTasks,
			subtask: subtasks
				.filter(s => noDueDateIds.has(s.id))
				.map(s => {
					const full = tasks.value.find(t => t.id === s.id)
					return full ? filterSubtasks(full, noDueDateIds) : s
				}),
		},
	}
}

const noDueDateTasks = computed(() => {
	const noDueDateIds = new Set(tasks.value.filter(t => !t.dueDate).map(t => t.id))
	const taskIds = new Set(tasks.value.map(t => t.id))
	return tasks.value
		.filter(t => {
			if (t.dueDate) return false
			// Hide if a parent task is also in this view (same logic as shouldShowTaskInListView)
			const parentIds = t.relatedTasks?.parenttask?.map(p => p.id) ?? []
			return parentIds.length === 0 || !parentIds.some(id => taskIds.has(id))
		})
		.map(t => filterSubtasks(t, noDueDateIds))
})

// — Day selection —
const selectedDate = ref<Date | null>(null)

function selectDay(date: Date) {
	selectedDate.value = isSameDay(date, selectedDate.value ?? new Date(0))
		? null
		: date
}

// — Task actions —
function openTask(task: ITask) {
	router.push({name: 'task.detail', params: {id: task.id}})
}

const newTaskTitle = ref('')
const addingTask = ref(false)
const taskService = new TaskService()

async function addTaskForDay() {
	if (!newTaskTitle.value || !selectedDate.value) return
	addingTask.value = true
	try {
		const dueDate = new Date(selectedDate.value)
		dueDate.setHours(23, 59, 0, 0)
		await taskService.create(new TaskModel({
			title: newTaskTitle.value,
			projectId: props.projectId,
			dueDate,
		}))
		newTaskTitle.value = ''
		await loadTasks()
	} finally {
		addingTask.value = false
	}
}

async function toggleDone(task: ITask) {
	await taskService.update(new TaskModel({...task, done: !task.done}))
	await loadTasks()
}
</script>

<style lang="scss" scoped>
.calendar-container {
	padding: 1rem;
	display: flex;
	flex-direction: column;
	gap: 1rem;
}

.calendar-nav {
	display: flex;
	align-items: center;
	gap: .5rem;

	&__title {
		font-size: 1.2rem;
		font-weight: 600;
		min-width: 14rem;
		text-align: center;
	}
}

.calendar-grid {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	gap: 2px;

	&__header {
		text-align: center;
		font-weight: 600;
		font-size: .85rem;
		padding: .25rem 0;
		color: var(--grey-500);
	}

	&__cell {
		min-height: 6rem;
		border: 1px solid var(--grey-200);
		border-radius: var(--radius);
		padding: .25rem;
		cursor: pointer;
		background: var(--white);
		transition: background .1s;

		&:hover {
			background: var(--grey-100);
		}

		&.is-other-month {
			opacity: .4;
		}

		&.is-today .calendar-grid__day-number {
			background: var(--primary);
			color: var(--white);
			border-radius: 50%;
			width: 1.6rem;
			height: 1.6rem;
			display: flex;
			align-items: center;
			justify-content: center;
		}

		&.is-selected {
			border-color: var(--primary);
			box-shadow: 0 0 0 2px var(--primary-light);
		}
	}

	&__day-number {
		font-size: .85rem;
		font-weight: 500;
		display: inline-block;
		margin-bottom: .15rem;
	}

	&__tasks {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	&__task-chip {
		font-size: .7rem;
		padding: 1px 4px;
		border-radius: 3px;
		background: var(--primary-light);
		color: var(--primary-dark);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		cursor: pointer;

		&.is-done {
			text-decoration: line-through;
			opacity: .6;
		}

		&.is-overflow {
			background: var(--grey-200);
			color: var(--grey-600);
			cursor: default;
		}
	}
}

.calendar-day-panel {
	border: 1px solid var(--grey-200);
	border-radius: var(--radius);
	padding: 1rem;
	background: var(--white);

	&__header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: .75rem;
	}

	&__add {
		display: flex;
		gap: .5rem;
		margin-bottom: .75rem;
	}

	&__task-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: .25rem;
	}

	&__task {
		display: flex;
		align-items: center;
		gap: .5rem;
	}

	&__task-title {
		cursor: pointer;

		&.is-done {
			text-decoration: line-through;
			opacity: .6;
		}

		&:hover {
			text-decoration: underline;
		}
	}
}

.no-due-date-section {
	border: 1px solid var(--grey-200);
	border-radius: var(--radius);
	padding: 1rem;
	background: var(--white);

	&__title {
		font-size: 1rem;
		font-weight: 600;
		margin-bottom: .75rem;
		color: var(--grey-600);
	}
}
</style>
