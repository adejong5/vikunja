<template>
	<div class="calendar-container">
		<div class="calendar-nav">
			<XButton
				variant="secondary"
				icon="angle-left"
				@click="prevMonth"
			>
				{{ $t('misc.previous') }}
			</XButton>
			<span class="calendar-nav__title">{{ monthTitle }}</span>
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
						v-for="item in cellItemsForDay(cell.date).slice(0, 3)"
						:key="item.id"
						class="calendar-grid__task-chip"
						:class="{'is-done': item.done, 'is-google': item.type === 'google'}"
						:title="item.title"
						@click.stop="item.taskData ? openTask(item.taskData) : undefined"
					>
						{{ item.title }}
					</span>
					<span
						v-if="cellItemsForDay(cell.date).length > 3"
						class="calendar-grid__task-chip is-overflow"
					>
						+{{ cellItemsForDay(cell.date).length - 3 }}
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

			<p
				v-if="tasksForDay(selectedDate).length === 0 && googleEventsForDay(selectedDate).length === 0"
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
					<div class="calendar-day-panel__task-info">
						<span
							class="calendar-day-panel__task-title"
							:class="{'is-done': task.done}"
							@click="openTask(task)"
						>
							{{ task.title }}
						</span>
						<span
							v-if="projectStore.projects[task.projectId]"
							class="calendar-day-panel__task-project"
						>
							{{ projectStore.projects[task.projectId].title }}
						</span>
					</div>
				</li>
			</ul>

			<ul
				v-if="googleEventsForDay(selectedDate).length > 0"
				class="calendar-day-panel__task-list mbs-4"
			>
				<li
					v-for="event in googleEventsForDay(selectedDate)"
					:key="event.id"
					class="calendar-day-panel__task"
				>
					<span
						class="calendar-day-panel__google-icon"
						:title="$t('project.calendar.googleSource')"
					>
						<svg
							viewBox="0 0 24 24"
							xmlns="http://www.w3.org/2000/svg"
							aria-hidden="true"
						>
							<path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
							<path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
							<path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
							<path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
						</svg>
					</span>
					<div class="calendar-day-panel__event-info">
						<span class="calendar-day-panel__task-title">{{ event.title }}</span>
						<span
							v-if="!event.allDay"
							class="calendar-day-panel__event-time"
						>
							{{ formatEventTime(event) }}
						</span>
					</div>
				</li>
			</ul>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useRouter} from 'vue-router'

import type {ITask} from '@/modelTypes/ITask'
import type {TaskFilterParams} from '@/services/taskCollection'

import {useTaskStore} from '@/stores/tasks'
import {useProjectStore} from '@/stores/projects'
import {useConfigStore} from '@/stores/config'
import {useTitle} from '@/composables/useTitle'
import {useI18n} from 'vue-i18n'
import {formatDateLong} from '@/helpers/time/formatDate'

import TaskService from '@/services/task'
import TaskModel from '@/models/task'
import GoogleCalendarService from '@/services/googleCalendar'
import type {GoogleCalendarEvent} from '@/services/googleCalendar'

import XButton from '@/components/input/Button.vue'

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('navigation.calendar'))

const router = useRouter()
const taskStore = useTaskStore()
const projectStore = useProjectStore()
const configStore = useConfigStore()
const googleCalendarEnabled = computed(() => configStore.googleCalendarEnabled)

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
	new Date(displayYear.value, displayMonth.value, 1)
		.toLocaleDateString(undefined, {month: 'long', year: 'numeric'}),
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
		cells.push({key: d.toISOString(), date: d, currentMonth: true, isToday: isSameDay(d, today)})
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

// — Task loading —
const tasks = ref<ITask[]>([])
const loadingTasks = ref(false)

async function loadTasks() {
	loadingTasks.value = true
	const start = new Date(displayYear.value, displayMonth.value, 1)
	const end = new Date(displayYear.value, displayMonth.value + 1, 1)

	const params: TaskFilterParams = {
		sort_by: ['due_date', 'id'],
		order_by: ['asc', 'desc'],
		filter: `due_date >= '${start.toISOString()}' && due_date < '${end.toISOString()}'`,
		filter_include_nulls: false,
		s: '',
		expand: ['comment_count', 'is_unread'],
	}

	try {
		tasks.value = await taskStore.loadTasks(params) ?? []
	} catch {
		tasks.value = []
	} finally {
		loadingTasks.value = false
	}
}

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

// — Google Calendar events —
interface CalendarDisplayItem {
	type: 'task' | 'google'
	id: string | number
	title: string
	done?: boolean
	taskData?: ITask
}

const googleService = new GoogleCalendarService()
const googleEvents = ref<GoogleCalendarEvent[]>([])

const googleEventsByDay = computed(() => {
	const map = new Map<string, GoogleCalendarEvent[]>()
	for (const event of googleEvents.value) {
		const start = new Date(event.start)
		if (event.allDay) {
			const end = new Date(event.end)
			const cur = new Date(start.getFullYear(), start.getMonth(), start.getDate())
			const endDay = new Date(end.getFullYear(), end.getMonth(), end.getDate())
			while (cur < endDay) {
				const key = dayKey(cur)
				if (!map.has(key)) map.set(key, [])
				map.get(key)!.push(event)
				cur.setDate(cur.getDate() + 1)
			}
		} else {
			const key = dayKey(start)
			if (!map.has(key)) map.set(key, [])
			map.get(key)!.push(event)
		}
	}
	return map
})

function googleEventsForDay(date: Date): GoogleCalendarEvent[] {
	return googleEventsByDay.value.get(dayKey(date)) ?? []
}

function cellItemsForDay(date: Date): CalendarDisplayItem[] {
	const taskItems: CalendarDisplayItem[] = tasksForDay(date).map(t => ({
		type: 'task' as const,
		id: t.id,
		title: t.title,
		done: t.done,
		taskData: t,
	}))
	const googleItems: CalendarDisplayItem[] = googleEventsForDay(date).map(e => ({
		type: 'google' as const,
		id: e.id,
		title: e.title,
	}))
	return [...taskItems, ...googleItems]
}

function formatEventTime(event: GoogleCalendarEvent): string {
	const fmt = (d: Date) => d.toLocaleTimeString(undefined, {hour: 'numeric', minute: '2-digit'})
	return `${fmt(new Date(event.start))} – ${fmt(new Date(event.end))}`
}

async function fetchGoogleEvents() {
	if (!googleCalendarEnabled.value) return
	const month = `${displayYear.value}-${String(displayMonth.value + 1).padStart(2, '0')}`
	try {
		googleEvents.value = await googleService.getUserEvents(month)
	} catch {
		googleEvents.value = []
	}
}

watch([displayYear, displayMonth], () => {
	loadTasks()
	fetchGoogleEvents()
}, {immediate: true})

// — Day selection —
const selectedDate = ref<Date | null>(null)

function selectDay(date: Date) {
	selectedDate.value = isSameDay(date, selectedDate.value ?? new Date(0)) ? null : date
}

// — Task actions —
function openTask(task: ITask) {
	router.push({name: 'task.detail', params: {id: task.id}})
}

const taskService = new TaskService()

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

		&:hover { background: var(--grey-100); }
		&.is-other-month { opacity: .4; }

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

		&.is-google {
			background: #e8f0fe;
			color: #1a73e8;
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
		align-items: flex-start;
		gap: .5rem;

		input[type='checkbox'] {
			margin-top: .2rem;
			flex-shrink: 0;
		}
	}

	&__task-info {
		display: flex;
		flex-direction: column;
		gap: .1rem;
		min-width: 0;
	}

	&__task-title {
		cursor: pointer;

		&.is-done {
			text-decoration: line-through;
			opacity: .6;
		}

		&:hover { text-decoration: underline; }
	}

	&__task-project {
		font-size: .75rem;
		color: var(--grey-500);
	}

	&__google-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1rem;
		height: 1rem;
		flex-shrink: 0;
		margin-top: .1rem;

		svg { width: 100%; height: 100%; }
	}

	&__event-info {
		display: flex;
		flex-direction: column;
		gap: .1rem;
		min-width: 0;
	}

	&__event-time {
		font-size: .75rem;
		color: var(--grey-500);
	}
}
</style>
