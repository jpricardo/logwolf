import { redirect } from 'react-router';

import { Page } from '~/components/nav/page';
import { eventContext } from '~/context';
import { allowlistFromEnv, checkInvitee, inviteWarning } from '~/lib/allowlist.server';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { lowersRetention } from '~/lib/retention';
import { commitSession, getSession } from '~/lib/session.server';

import type { Route } from './+types';
import { DangerZone } from './components/danger-zone';
import { GeneralSection } from './components/general-section';
import { MembersSection } from './components/members-section';
import { RetentionSection } from './components/retention-section';

export function meta({ data }: Route.MetaArgs) {
	return [{ title: `${data?.project.name ?? 'Project'} settings - Logwolf` }];
}

/**
 * Resolves the project in the URL against the projects the caller belongs to.
 * One list call answers both "may they open this page?" and "what role do they
 * hold?" — the broker would reject a non-member anyway, but as a 403 that only
 * the error boundary could render.
 */
async function requireProject(request: Request, id: string | undefined) {
	const user = await requireAuth(request);
	const api = createApi(user.login);

	const projects = await api.getProjects();
	const project = projects.find((p) => p.id === id);
	if (!project) throw redirect('/projects');

	return { user, api, project };
}

export async function loader({ request, params, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const { user, api, project } = await requireProject(request, params.id);

	const [members, retention] = await Promise.all([api.getMembers(project.id), api.getRetention(project.id)]);
	event?.set('loaderData', { project, memberCount: members.length, days: retention.days });

	return { project, members, days: retention.days, currentUser: user.login };
}

export async function action({ request, params, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const { api, project } = await requireProject(request, params.id);

	const fd = await request.formData();
	await validateCsrfToken(request, fd);

	const intent = fd.get('intent')?.toString() ?? '';
	event?.set('intent', intent);

	// Retention is the one setting any member may change, and only upwards; the
	// broker enforces the same split, but repeating it here turns a bare
	// "forbidden" from a stale tab into a message the page can show next to the
	// control.
	if (intent !== 'retention' && project.role !== 'owner') {
		return { error: 'Only an owner can change this.' };
	}

	try {
		if (intent === 'rename') {
			const name = fd.get('name')?.toString().trim() ?? '';
			if (!name) return { error: 'Name is required.' };

			// Only the name changes: the slug is fixed at creation.
			await api.updateProject(project.id, name);
			return { success: `Renamed to ${name}.` };
		}

		if (intent === 'retention') {
			const days = Number(fd.get('days'));
			if (project.role !== 'owner') {
				const current = await api.getRetention(project.id);
				if (lowersRetention(current.days, days)) return { error: 'Only an owner can lower retention.' };
			}

			const res = await api.updateRetention(project.id, days);
			event?.set('actionData', res);
			return { success: 'Retention updated.' };
		}

		if (intent === 'add-member') {
			const login = fd.get('login')?.toString().trim() ?? '';
			const role = fd.get('role')?.toString() === 'owner' ? 'owner' : 'member';
			if (!login) return { error: 'A GitHub login is required.' };

			// Adding someone who can never sign in would look like it worked, so
			// ask GitHub first. Only a login that cannot be a person is refused;
			// one the allowlist does not clear is added with a warning, since an
			// admin may allowlist them later.
			const check = await checkInvitee(login, allowlistFromEnv());
			event?.set('inviteCheck', check.kind);
			if (check.kind === 'unknown') return { error: `There is no GitHub user named ${login}.` };
			if (check.kind === 'organization') {
				return { error: `${check.login} is a GitHub organization; only users can be members.` };
			}

			await api.addMember(project.id, check.login, role);
			return { success: `Added ${check.login} as ${role}.`, warning: inviteWarning(check) };
		}

		if (intent === 'change-role') {
			const login = fd.get('login')?.toString() ?? '';
			const role = fd.get('role')?.toString();
			if (role !== 'owner' && role !== 'member') return { error: 'Choose owner or member.' };

			await api.updateMemberRole(project.id, login, role);
			return { success: `${login} is now ${role === 'owner' ? 'an owner' : 'a member'}.` };
		}

		if (intent === 'remove-member') {
			const login = fd.get('login')?.toString() ?? '';
			await api.removeMember(project.id, login);
			return { success: `Removed ${login}.` };
		}

		if (intent === 'delete') {
			// The dialog disables its button until the typed name matches, but the
			// check that counts is this one — a form post never sees that button.
			const confirmation = fd.get('confirmation')?.toString() ?? '';
			if (confirmation !== project.name) return { error: 'The project name does not match.' };

			await api.deleteProject(project.id);

			const session = await getSession(request.headers.get('Cookie'));
			if (session.get('currentProjectID') === project.id) session.unset('currentProjectID');

			// /projects renders inside the layout, which forwards to /projects/new
			// when the project just deleted was the caller's last one.
			return redirect('/projects', { headers: { 'Set-Cookie': await commitSession(session) } });
		}

		return null;
	} catch (err) {
		event?.setSeverity('error');
		event?.set('actionError', err);
		return { error: (err as Error).message };
	}
}

export default function ProjectSettings({ loaderData }: Route.ComponentProps) {
	const { project, members, days, currentUser } = loaderData;
	const isOwner = project.role === 'owner';

	return (
		<Page title={`${project.name} settings`}>
			<div className='flex flex-col gap-8'>
				<GeneralSection project={project} canEdit={isOwner} />
				<RetentionSection days={days} canLower={isOwner} />
				<MembersSection members={members} currentUser={currentUser} canManage={isOwner} />
				{isOwner && <DangerZone project={project} />}
			</div>
		</Page>
	);
}
