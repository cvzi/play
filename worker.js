/*
https://github.com/cvzi/play/

Copyright (C) 2022 cuzi

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.

Provides a badge for apps on the Google Play Store
The program parses the Play Store website and provides
data for https://shields.io/endpoint

This is run as a Cloudflare worker
If a KV storage is bound to the variable PLAY_CACHE, it will be
used to cache requests to play.google.com.

Example: https://play.cuzi.workers.dev/play?i=org.mozilla.firefox&l=Android&m=$version
Badge: https://img.shields.io/endpoint?url=https%3A%2F%2Fplay.cuzi.workers.dev%2Fplay%3Fi%3Dorg.mozilla.firefox%26l%3DAndroid%26m%3D%24version

*/

const appIDPattern = /[a-zA-Z0-9_]+\.[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)*/

const playStorePlaceHolders = {
  $version: 'App version',
  $installs: 'Installs',
  $totalinstalls: 'Precise installs',
  $shortinstalls: 'Shorter installs',
  $updated: 'Last update',
  $android: 'Required min. Android version',
  $targetandroid: 'Target Android version',
  $minsdk: 'Required min. SDK',
  $targetsdk: 'Target SDK',
  $rating: 'Rating',
  $floatrating: 'Precise rating',
  $name: 'Name',
  $friendly: 'Content Rating',
  $published: 'First published'
}

const fetchCacheSeconds = 60 * 60 * 5 // 5 hours
const kvCacheSeconds = 60 * 60 * 6 // 6 hours

const fetchConfig = {
  cf: {
    // Always cache this fetch regardless of content type
    // for a max of 5 hours before revalidating the resource
    cacheTtl: fetchCacheSeconds,
    cacheEverything: true
  }
}

const responseConfigJSON = {
  headers: {
    'content-type': 'application/json; charset=utf-8'
  }
}

const responseConfigHTML = {
  headers: {
    'content-type': 'text/html; charset=utf-8'
  }
}

async function cachedFetchText (url, fetchConfig, ctx) {
  if (typeof PLAY_CACHE !== 'undefined') {
    let data = null

    try {
      console.log(`Trying to find cache for ${url}`)
      data = PLAY_CACHE.get(url, { type: 'text' })
    } catch (e) {
      // Catch error 426 'Too many requests' on free plan
      console.warn(e)
    }

    if (!data) {
      console.log(`Cache for ${url} not found. Requesting data`)
      data = await (await fetch(url, fetchConfig)).text()
      try {
        console.log(`Saving data for url ${url} to cache`)
        ctx.waitUntil(PLAY_CACHE.put(url, data, { expirationTtl: kvCacheSeconds }))
      } catch (e) {
        // Catch error 426 'Too many requests' on free plan
        console.warn(e)
      }
    } else {
      console.log(`Cache hit for url ${url}`)
    }

    return data
  } else {
    console.log(`No cache, fetch ${url} directly`)
    return await (await fetch(url, fetchConfig)).text()
  }
}

function replaceVars (text, templateVars) {
  const escapeRegExp = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  for (const name in templateVars) {
    text = text.replace(new RegExp('\\{\\{\\s*' + escapeRegExp(name) + '\\s*\\}\\}', 'gm'), templateVars[name])
  }
  return text
}

async function template (templateUrl, templateVars) {
  let html = await (await fetch(templateUrl, fetchConfig)).text()

  html = html.replace(/\{\{items:(\w+)\}\}([\s\S]*?)\{\{end\}\}/gm, function (wholeMatch, name, body) {
    if (name in templateVars) {
      const result = []
      for (const key in templateVars[name]) {
        result.push(replaceVars(body, Object.assign({}, templateVars, { $k: key, $v: templateVars[name][key] })))
      }
      return result.join('\n')
    } else {
      return wholeMatch
    }
  })

  html = replaceVars(html, templateVars)

  return new Response(html, responseConfigHTML)
}

function replacePlaceHolders (str, app) {
  for (const placeholder in playStorePlaceHolders) {
    str = str.replace(placeholder, app[placeholder.substring(1)])
  }
  return str
}

function getOrDefault (obj, indices, fallback = '', post = (x) => x) {
  let i
  try {
    for (i = 0; i < indices.length; i++) {
      obj = obj[indices[i]]
    }
    if (obj != null) {
      return post(obj)
    }
  } catch (e) {
    console.warn(`at i=${i} in ${indices}`, e, '\nin obj:', obj)
  }
  return fallback
}

function language (hl, gl) {
  const languageQuery = []
  if (hl) {
    languageQuery.push(`hl=${hl}`)
  }
  if (gl) {
    languageQuery.push(`gl=${gl}`)
  }

  if (languageQuery.length === 0) return ''

  return `&${languageQuery.join('&')}`
}

async function getPlayStore (packageName, env, ctx, hl, gl) {
  const lang = language(hl, gl)
  const url = `https://play.google.com/store/apps/details?id=${encodeURIComponent(packageName)}${lang}`
  const content = await cachedFetchText(url, fetchConfig, ctx)

  const parts = content.split('AF_initDataCallback({').slice(1).map(v => v.split('</script>')[0])
  if (parts.length === 0) {
    throw new Error(`Failed to extract data from play store [${packageName}]`)
  }

  const dataString = parts.filter(s => s.indexOf(`["${packageName}"],`) !== -1)[0].trim()
  const noDataString = dataString.substring(dataString.indexOf('data:') + 5)
  let arrString = noDataString.split('sideChannel:')[0].trim()
  arrString = arrString.substring(0, arrString.length - 1) // remove trailing comma

  const json = JSON.parse(arrString)

  const fallback = 'Varies with device'

  const result = {
    name: getOrDefault(json, [1, 2, 0], fallback),
    installs: getOrDefault(json, [1, 2, 13, 0], fallback),
    totalinstalls: getOrDefault(json, [1, 2, 13, 2], fallback, (n) => n.toLocaleString()),
    shortinstalls: getOrDefault(json, [1, 2, 13, 3], fallback),
    version: getOrDefault(json, [1, 2, 140, 0, 0], fallback),
    updated: getOrDefault(json, [1, 2, 145, 0, 0], fallback),
    targetandroid: getOrDefault(json, [1, 2, 140, 1, 0, 0, 1], fallback),
    targetsdk: getOrDefault(json, [1, 2, 140, 1, 0, 0, 0], fallback),
    android: getOrDefault(json, [1, 2, 140, 1, 1, 0, 0, 1], fallback),
    minsdk: getOrDefault(json, [1, 2, 140, 1, 1, 0, 0, 0], fallback),
    rating: getOrDefault(json, [1, 2, 51, 0, 0], fallback),
    floatrating: getOrDefault(json, [1, 2, 51, 0, 1], fallback),
    friendly: getOrDefault(json, [1, 2, 9, 0], fallback),
    published: getOrDefault(json, [1, 2, 10, 0], fallback)
  }

  return result
}

function errorJSON (message) {
  return new Response(JSON.stringify({
    schemaVersion: 1,
    label: 'error',
    message: '' + message,
    isError: true
  }), responseConfigJSON)
}

async function handleBadge (env, ctx, url) {
  const appId = url.searchParams.get('i') || url.searchParams.get('id') || ''

  if (!appId) {
    return errorJSON('missing app id')
  }

  const m = appId.match(appIDPattern)
  if (!m || !m[0]) {
    return errorJSON('invalid app id format')
  }

  const hl = url.searchParams.get('hl')
  const gl = url.searchParams.get('gl')

  let playData
  try {
    playData = await getPlayStore(m[0], env, ctx, hl, gl)
  } catch (e) {
    console.error(e)
    return errorJSON(e)
  }

  let label = url.searchParams.get('l') || url.searchParams.get('label') || 'play'
  let message = url.searchParams.get('m') || url.searchParams.get('message') || '$version'

  label = replacePlaceHolders(label.substring(0, 1000), playData)
  message = replacePlaceHolders(message.substring(0, 1000), playData)

  return new Response(JSON.stringify({
    schemaVersion: 1,
    label,
    message,
    cacheSeconds: 3600
  }), responseConfigJSON)
}

function handleIndex (url) {
  const templateVars = {
    appid: url.searchParams.get('i') || url.searchParams.get('id') || 'org.mozilla.firefox',
    label: url.searchParams.get('l') || url.searchParams.get('label') || 'Android',
    message: url.searchParams.get('m') || url.searchParams.get('message') || '$version',
    displayLanguage: url.searchParams.get('hl') || 'en',
    playStoreCountry: url.searchParams.get('gl') || 'US',
    placeHolders: playStorePlaceHolders
  }

  return template('https://cvzi.github.io/play/index.html', templateVars)
}

async function handleRequest (request, env, ctx) {
  const url = new URL(request.url)
  if (url.pathname.startsWith('/play')) {
    return handleBadge(env, ctx, url)
  } else if (url.pathname.startsWith('/favicon')) {
    return Response.redirect('https://cvzi.github.io/play/favicon.ico', 301)
  } else {
    return handleIndex(url)
  }
}

export default {
  async fetch (request, env, ctx) {
    return handleRequest(request, env, ctx)
  }
}
