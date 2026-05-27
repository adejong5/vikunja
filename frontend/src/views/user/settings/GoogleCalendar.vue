<template>
	<Card :title="$t('user.settings.googleCalendar.title')">
		<p v-if="!googleCalendarEnabled">
			{{ $t('user.settings.googleCalendar.notEnabled') }}
		</p>

		<template v-else>
			<!-- Link status -->
			<div class="gcal-status mbe-4">
				<template v-if="status?.linked">
					<span class="gcal-status__badge is-linked">
						<icon icon="check" />
						{{ $t('user.settings.googleCalendar.linked') }}
					</span>
					<span
						v-if="status.linkedAt"
						class="gcal-status__since"
					>
						{{ $t('user.settings.googleCalendar.linkedSince', {date: formatDateLong(status.linkedAt)}) }}
					</span>
				</template>
				<span
					v-else
					class="gcal-status__badge is-unlinked"
				>
					{{ $t('user.settings.googleCalendar.notLinked') }}
				</span>
			</div>

			<!-- Settings (only shown when linked) -->
			<div
				v-if="status?.linked"
				class="gcal-settings mbe-4"
			>
				<div class="gcal-settings__row">
					<input
						id="showInVikunja"
						v-model="showInVikunja"
						type="checkbox"
						class="checkbox"
						:disabled="saving"
						@change="saveSettings"
					/>
					<div>
						<label
							for="showInVikunja"
							class="gcal-settings__label"
						>
							{{ $t('user.settings.googleCalendar.showInVikunja') }}
						</label>
						<p class="gcal-settings__hint">
							{{ $t('user.settings.googleCalendar.showInVikunjaHint') }}
						</p>
					</div>
				</div>

				<div class="gcal-settings__row">
					<input
						id="showVikunjaInGoogle"
						v-model="showVikunjaInGoogle"
						type="checkbox"
						class="checkbox"
						disabled
					/>
					<div>
						<label
							for="showVikunjaInGoogle"
							class="gcal-settings__label has-text-grey"
						>
							{{ $t('user.settings.googleCalendar.showVikunjaInGoogle') }}
						</label>
						<p class="gcal-settings__hint">
							{{ $t('user.settings.googleCalendar.showVikunjaInGoogleHint') }}
						</p>
					</div>
				</div>
			</div>

			<!-- Actions -->
			<div class="gcal-actions">
				<XButton
					v-if="!status?.linked"
					:loading="linking"
					icon="link"
					@click="linkAccount"
				>
					{{ $t('user.settings.googleCalendar.linkAccount') }}
				</XButton>

				<XButton
					v-else
					variant="secondary"
					:loading="unlinking"
					icon="times"
					@click="unlinkAccount"
				>
					{{ $t('user.settings.googleCalendar.unlinkAccount') }}
				</XButton>
			</div>
		</template>
	</Card>
</template>

<script setup lang="ts">
import {ref, onMounted, computed} from 'vue'
import {useRoute} from 'vue-router'
import {useI18n} from 'vue-i18n'

import {useTitle} from '@/composables/useTitle'
import {useConfigStore} from '@/stores/config'
import {success, error} from '@/message'
import {formatDateLong} from '@/helpers/time/formatDate'

import XButton from '@/components/input/Button.vue'
import GoogleCalendarService from '@/services/googleCalendar'
import type {GoogleCalendarStatus} from '@/services/googleCalendar'

const {t} = useI18n({useScope: 'global'})
useTitle(() => `${t('user.settings.googleCalendar.title')} - ${t('user.settings.title')}`)

const configStore = useConfigStore()
const googleCalendarEnabled = computed(() => configStore.googleCalendarEnabled)

const service = new GoogleCalendarService()
const status = ref<GoogleCalendarStatus | null>(null)
const showInVikunja = ref(false)
const showVikunjaInGoogle = ref(false)
const linking = ref(false)
const unlinking = ref(false)
const saving = ref(false)

async function loadStatus() {
	try {
		status.value = await service.getStatus()
		showInVikunja.value = status.value.showInVikunja
		showVikunjaInGoogle.value = status.value.showVikunjaInGoogle
	} catch (e) {
		error(e)
	}
}

async function linkAccount() {
	linking.value = true
	try {
		const url = await service.getAuthUrl()
		window.location.href = url
	} catch (e) {
		error(e)
		linking.value = false
	}
}

async function unlinkAccount() {
	unlinking.value = true
	try {
		await service.unlink()
		success({message: t('user.settings.googleCalendar.unlinkSuccess')})
		await loadStatus()
	} catch (e) {
		error(e)
	} finally {
		unlinking.value = false
	}
}

async function saveSettings() {
	saving.value = true
	try {
		await service.updateSettings({
			showInVikunja: showInVikunja.value,
			showVikunjaInGoogle: showVikunjaInGoogle.value,
		})
		success({message: t('user.settings.googleCalendar.settingsSaved')})
	} catch (e) {
		error(e)
	} finally {
		saving.value = false
	}
}

// Handle redirect back from Google OAuth with ?linked=true
const route = useRoute()
onMounted(async () => {
	await loadStatus()
	if (route.query.linked === 'true') {
		success({message: t('user.settings.googleCalendar.linkSuccess')})
	}
})
</script>

<style scoped lang="scss">
.gcal-status {
	display: flex;
	align-items: center;
	gap: .75rem;

	&__badge {
		display: inline-flex;
		align-items: center;
		gap: .4rem;
		padding: .25rem .75rem;
		border-radius: 9999px;
		font-size: .85rem;
		font-weight: 600;

		&.is-linked {
			background: var(--success-light, #d4edda);
			color: var(--success, #28a745);
		}

		&.is-unlinked {
			background: var(--grey-200);
			color: var(--grey-600);
		}
	}

	&__since {
		font-size: .85rem;
		color: var(--grey-500);
	}
}

.gcal-settings {
	display: flex;
	flex-direction: column;
	gap: 1rem;

	&__row {
		display: flex;
		align-items: flex-start;
		gap: .75rem;

		.checkbox {
			margin-top: .2rem;
			flex-shrink: 0;
		}
	}

	&__label {
		font-weight: 500;
		cursor: pointer;
		display: block;
	}

	&__hint {
		font-size: .85rem;
		color: var(--grey-500);
		margin: .15rem 0 0;
	}
}

.gcal-actions {
	display: flex;
	gap: .5rem;
}
</style>
