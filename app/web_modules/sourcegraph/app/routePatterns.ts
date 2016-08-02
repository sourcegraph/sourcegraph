import {matchPattern} from "react-router/lib/PatternUtils";

export type RouteName = "styleguide" |
	"search" |
	"home" |
	"integrations" |
	"tool" |
	"settings" |
	"settingsRepos" |
	"commit" |
	"def" |
	"defInfo" |
	"desktopHome" |
	"repo" |
	"tree" |
	"blob" |
	"build" |
	"builds" |
	"login" |
	"signup" |
	"forgot" |
	"reset" |
	"about" |
	"beta" |
	"contact" |
	"security" |
	"pricing" |
	"terms" |
	"privacy" |
	"admin" |
	"adminBuilds" |
	"coverage" |
	"adminCoverage" |
	"browserExtFaqs";

// NOTE: If you add a top-level route (e.g., "/about"), add it to the
// topLevel list in app/internal/ui/router.go.
export const rel = {
	search: "search",
	about: "about",
	beta: "beta",
	browserExtFaqs: "about/browser-ext-faqs",
	contact: "contact",
	desktopHome: "desktop/home",
	security: "security",
	pricing: "pricing",
	terms: "-/terms",
	privacy: "-/privacy",
	styleguide: "styleguide",
	home: "",
	integrations: "integrations",
	settings: "settings/",
	settingsRepos: "repos",
	login: "login",
	signup: "join",
	forgot: "forgot",
	reset: "reset",
	admin: "-/",
	commit: "commit",
	def: "def/*",
	defInfo: "info/*",
	repo: "*", // matches both "repo" and "repo@rev"
	tree: "tree/*",
	blob: "blob/*",
	build: "builds/:id",
	builds: "builds",
	coverage: "coverage",
};

export const abs = {
	search: rel.search,
	about: rel.about,
	contact: rel.contact,
	desktopHome: rel.desktopHome,
	security: rel.security,
	browserExtFaqs: rel.browserExtFaqs,
	pricing: rel.pricing,
	terms: rel.terms,
	privacy: rel.privacy,
	styleguide: rel.styleguide,
	home: rel.home,
	integrations: rel.integrations,
	settings: rel.settings,
	settingsRepos: `${rel.settings}/${rel.settingsRepos}`,
	login: rel.login,
	signup: rel.signup,
	forgot: rel.forgot,
	reset: rel.reset,
	admin: rel.admin,
	adminBuilds: `${rel.admin}${rel.builds}`,
	adminCoverage: `${rel.admin}${rel.coverage}`,
	commit: `${rel.repo}/-/${rel.commit}`,
	def: `${rel.repo}/-/${rel.def}`,
	defInfo: `${rel.repo}/-/${rel.defInfo}`,
	repo: rel.repo,
	tree: `${rel.repo}/-/${rel.tree}`,
	blob: `${rel.repo}/-/${rel.blob}`,
	build: `${rel.repo}/-/${rel.build}`,
	builds: `${rel.repo}/-/${rel.builds}`,
};

const routeNamesByPattern: {[key: string]: RouteName} = {};
for (let name of Object.keys(abs)) {
	routeNamesByPattern[abs[name]] = name as RouteName;
}

export function getRoutePattern(routes: Array<ReactRouter.PlainRoute>): string {
	return routes.map((route) => route.path).join("").slice(1); // remove leading '/''
}

export function getRouteName(routes: Array<ReactRouter.PlainRoute>): string | null {
	return routeNamesByPattern[getRoutePattern(routes)] || null;
}

export function getViewName(routes: Array<ReactRouter.PlainRoute>): string | null {
	let name = getRouteName(routes);
	if (name) {
		return `View${name.charAt(0).toUpperCase()}${name.slice(1)}`;
	}
	return null;
}

export function getRouteParams(pattern: string, pathname: string): any {
	if (pathname.charAt(0) !== "/") { pathname = `/${pathname}`; }
	const {paramNames, paramValues} = matchPattern(pattern, pathname);

	if (paramValues !== null) {
		return paramNames.reduce((memo, paramName, index) => {
			if (typeof memo[paramName] === "undefined") {
				memo[paramName] = paramValues[index];
			} else if (typeof memo[paramName] === "string") {
				memo[paramName] = [memo[paramName], paramValues[index]];
			} else {
				memo[paramName].push(paramValues[index]);
			}
			return memo;
		}, {});
	}

	return null;
}
