// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	// Deployed to GitHub Pages under the repository sub-path.
	site: 'https://jvcorredor.github.io',
	base: '/srs-tui',
	integrations: [
		starlight({
			title: 'srs',
			description:
				'A spaced-repetition TUI built around the FSRS algorithm and Markdown card files.',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/jvcorredor/srs-tui',
				},
			],
			// "Edit page" links point at the docs/ directory in the repo.
			editLink: {
				baseUrl: 'https://github.com/jvcorredor/srs-tui/edit/main/docs/',
			},
			// Show the last-updated date, derived from git history.
			lastUpdated: true,
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Overview', link: '/' },
						{ slug: 'install' },
						{ slug: 'quick-start' },
					],
				},
			],
		}),
	],
});
