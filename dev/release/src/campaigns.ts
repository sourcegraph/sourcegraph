import { CreatedChangeset } from './github'
import { readLine, cacheFolder } from './util'
import YAML from 'yaml'
import execa from 'execa'
import fetch from 'node-fetch'
import commandExists from 'command-exists'

// https://about.sourcegraph.com/handbook/engineering/deployments/instances#k8s-sgdev-org
const DEFAULT_SRC_ENDPOINT = 'https://k8s.sgdev.org'

interface SourcegraphCLIConfig {
    SRC_ENDPOINT: string
    SRC_ACCESS_TOKEN: string
    [index: string]: string
}

/**
 * Retrieves src-cli configuration and ensures src-cli exists.
 */
export async function sourcegraphCLIConfig(): Promise<SourcegraphCLIConfig> {
    await commandExists('src') // CLI must be present for campaigns interactions
    return {
        SRC_ENDPOINT: DEFAULT_SRC_ENDPOINT,
        SRC_ACCESS_TOKEN: await readLine('k8s.sgdev.org src-cli token: ', `${cacheFolder}/src-cli.txt`),
    }
}

/**
 * Parameters defining a campaign to interact with.
 *
 * Generate `cliConfig` using `sourcegraphCLIConfig()`.
 */
export interface CampaignOptions {
    name: string
    description: string
    namespace: string
    cliConfig: SourcegraphCLIConfig
}

/**
 * Generate campaign configuration for a given release.
 */
export function releaseTrackingCampaign(version: string, cliConfig: SourcegraphCLIConfig): CampaignOptions {
    return {
        name: `release-sourcegraph-${version}`,
        description: `Track publishing of sourcegraph@${version}`,
        namespace: 'sourcegraph',
        cliConfig,
    }
}

/**
 * Generates a URL for a campaign that would be created under the given camapign options.
 *
 * Does not ensure the campaign exists.
 */
export function campaignURL(options: CampaignOptions): string {
    return `${options.cliConfig.SRC_ENDPOINT}/organizations/${options.namespace}/campaigns/${options.name}`
}

/**
 * Create a new campaign from a set of changes.
 */
export async function createCampaign(changes: CreatedChangeset[], options: CampaignOptions): Promise<void> {
    // create a campaign spec
    const importChangesets = changes.map(change => ({
        repository: `github.com/${change.repository}`,
        externalIDs: [change.pullRequestNumber],
    }))
    // apply campaign
    return await applyCampaign(
        {
            name: options.name,
            description: options.description,
            importChangesets,
        },
        options
    )
}

/**
 * Append changes to an existing campaign.
 */
export async function addToCampaign(
    changes: { repository: string; pullRequestNumber: number }[],
    options: CampaignOptions
): Promise<void> {
    const response = await fetch(`${options.cliConfig.SRC_ENDPOINT}/.api/graphql`, {
        method: 'POST',
        headers: {
            Authorization: `token ${options.cliConfig.SRC_ACCESS_TOKEN}`,
        },
        body: JSON.stringify({
            query: `query getCampaigns($namespace:String!) {
                organization(name:$namespace) {
                  campaigns(first:99) {
                    nodes { name currentSpec { originalInput } }
                  }
                }
              }`,
            variables: {
                namespace: options.namespace,
            },
        }),
    })
    const {
        data: {
            organization: {
                campaigns: { nodes: results },
            },
        },
    } = (await response.json()) as {
        data: { organization: { campaigns: { nodes: { name: string; currentSpec: { originalInput: string } }[] } } }
    }
    const campaign = results.find(result => result.name === options.name)
    if (!campaign) {
        throw new Error(`Cannot find campaign ${options.name}`)
    }

    const importChangesets = changes.map(change => ({
        repository: `github.com/${change.repository}`,
        externalIDs: [change.pullRequestNumber],
    }))
    const newSpec = YAML.parse(campaign.currentSpec.originalInput) as CampaignSpec
    newSpec.importChangesets.push(...importChangesets)
    await applyCampaign(newSpec, options)
}

/**
 * Subset of campaign spec: https://docs.sourcegraph.com/batch_changes/references/batch_spec_yaml_reference
 */
interface CampaignSpec {
    name: string
    description: string
    importChangesets: { repository: string; externalIDs: number[] }[]
}

async function applyCampaign(campaign: CampaignSpec, options: CampaignOptions): Promise<void> {
    const campaignYAML = YAML.stringify(campaign)
    console.log(`Rendered campaign spec:\n\n${campaignYAML}`)

    // apply the campaign
    await execa('src', ['campaign', 'apply', '-namespace', options.namespace, '-f', '-'], {
        stdout: 'inherit',
        input: campaignYAML,
        env: options.cliConfig,
    })
}
