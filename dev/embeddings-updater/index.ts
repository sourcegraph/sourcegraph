import * as fs from 'fs'
const fetch = require('node-fetch')

interface Repo {
    name: string;
    url: string; 
}
  
interface EmbedddingJob {
    id: string;
    state: string;
    repo: Repo;
}

const endpoint = 'https://sourcegraph.com/.api/graphql'

async function gqlRequest(endpoint: string) {

    try {
        let pagination = true
        let endCursor = ""
        const embeddedRepos: EmbedddingJob[] = []
        
        const access_token = `${process.env.access_token}`
        while (pagination) {
            console.log(`endcursor: ${endCursor}`)
            let query = `
                query RepoEmbeddingJobs {
                    repoEmbeddingJobs(first: 100, after: ${endCursor ? '"' + endCursor + '"' : null}) {
                    totalCount
                    pageInfo {
                        endCursor
                        hasNextPage
                    }
                    nodes {
                        id
                        state
                        repo {
                        name
                        url
                        }
                    }
                }
            }`
            const response = await fetch(endpoint, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `token ${access_token}`
                },
                body: JSON.stringify({query})
            })
            const {data} = await response.json()
            embeddedRepos.push(...data.repoEmbeddingJobs.nodes)
            pagination = data.repoEmbeddingJobs.pageInfo.hasNextPage
            endCursor = data.repoEmbeddingJobs.pageInfo.endCursor
        }
        const filtered = embeddedRepos.filter(item => item.state === 'COMPLETED')
        embeddedReposToMarkdown(filtered)
    } catch (err) {
        console.error(err)
    }
    
}

function embeddedReposToMarkdown(repos: EmbedddingJob[]) {
    const today = new Date();

    let markdown = `# Embeddings for repositories with 5+ stars\n\n`;
    markdown += `Last updated: ${today.toLocaleString('en-US', { 
        month: '2-digit', 
        day: '2-digit', 
        year: 'numeric',
        hour: '2-digit', 
        minute: '2-digit',
        timeZoneName: 'short' 
      })} \n\n`;  
    

    repos.forEach(repo => {
        markdown += `1. [${repo.repo.name}](${repo.repo.url})\n`
    })

    fs.writeFileSync('embeddedRepos.md', markdown)
}
 

gqlRequest(endpoint)