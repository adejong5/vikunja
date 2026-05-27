import {HTTPFactory} from '@/helpers/fetcher'

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

export default class GoogleCalendarService {
	private http = HTTPFactory()

	async getStatus(): Promise<GoogleCalendarStatus> {
		const {data} = await this.http.get<GoogleCalendarStatus>('/user/settings/google')
		return data
	}

	async getAuthUrl(): Promise<string> {
		const {data} = await this.http.get<{url: string}>('/user/settings/google/auth-url')
		return data.url
	}

	async updateSettings(settings: GoogleCalendarSettings): Promise<void> {
		await this.http.patch('/user/settings/google', settings)
	}

	async unlink(): Promise<void> {
		await this.http.delete('/user/settings/google')
	}
}
