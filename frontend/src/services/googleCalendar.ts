import {AuthenticatedHTTPFactory} from '@/helpers/fetcher'
import {objectToCamelCase, objectToSnakeCase} from '@/helpers/case'

export interface GoogleCalendarStatus {
	enabled: boolean
	linked: boolean
	linkedAt?: string
	showInVikunja: boolean
	showVikunjaInGoogle: boolean
}

export interface GoogleCalendarSettings {
	showInVikunja: boolean
	showVikunjaInGoogle: boolean
}

export interface GoogleCalendarEvent {
	id: string
	title: string
	start: string   // ISO 8601
	end: string     // ISO 8601
	allDay: boolean
	calendarName: string
}

export default class GoogleCalendarService {
	private http = AuthenticatedHTTPFactory()

	async getStatus(): Promise<GoogleCalendarStatus> {
		const {data} = await this.http.get<GoogleCalendarStatus>('/user/settings/google')
		return objectToCamelCase(data) as GoogleCalendarStatus
	}

	async getAuthUrl(): Promise<string> {
		const {data} = await this.http.get<{url: string}>('/user/settings/google/auth-url')
		return data.url
	}

	async updateSettings(settings: GoogleCalendarSettings): Promise<void> {
		await this.http.patch('/user/settings/google', objectToSnakeCase(settings))
	}

	async unlink(): Promise<void> {
		await this.http.delete('/user/settings/google')
	}

	async getEvents(projectId: number, viewId: number, month: string): Promise<GoogleCalendarEvent[]> {
		const {data} = await this.http.get<GoogleCalendarEvent[]>(
			`/projects/${projectId}/views/${viewId}/google-events`,
			{params: {month}},
		)
		return (data ?? []).map(e => objectToCamelCase(e) as GoogleCalendarEvent)
	}
}
