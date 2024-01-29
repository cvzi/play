if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', main)
} else {
  main()
}

function main (ev) {
  if (document.location.protocol !== 'https:' && document.location.hostname.indexOf('heroku') !== -1) {
    document.location.protocol = 'https:'
  }
  if (!document.getElementById('appid').value) {
    document.getElementById('appid').value = document.getElementById('appid').placeholder.split(' ')[0]
  }
  document.getElementById('baseurl').value = document.location.origin + document.getElementById('basepath').value
  document.getElementById('baseurl').addEventListener('change', updateUrls)
  document.getElementById('baseendpoint').addEventListener('change', updateUrls)
  document.getElementById('appid').addEventListener('change', updateUrls)
  document.getElementById('gl').addEventListener('change', updateUrls)
  document.getElementById('hl').addEventListener('change', updateUrls)
  document.getElementById('label').addEventListener('change', updateUrls)
  document.getElementById('message').addEventListener('change', updateUrls)
  document.getElementById('baseurl').addEventListener('keyup', updateUrls)
  document.getElementById('baseendpoint').addEventListener('keyup', updateUrls)
  document.getElementById('appid').addEventListener('keyup', updateUrls)
  document.getElementById('label').addEventListener('keyup', updateUrls)
  document.getElementById('message').addEventListener('keyup', updateUrls)
  document.getElementById('shieldurl').addEventListener('change', updateImages)
  document.getElementById('shieldurlstyled').addEventListener('change', updateImages)
  document.getElementById('go').addEventListener('click', function (ev) { ev.preventDefault(); updateUrls(ev)})
  document.querySelectorAll('.copy').forEach(el => el.addEventListener('click', copyMenu))
  document.getElementById('copymenu').addEventListener('click', ev => ev.stopPropagation())
  document.body.addEventListener('click', function() {
    const m = document.getElementById('copymenu')
    if(m) m.style.display = 'none'
  })
  updateUrls()
  toggleMobile()
}

function composeJsonUrl() {
  const base = document.getElementById('baseurl').value
  const endpoint = document.getElementById('baseendpoint').value
  let appid = document.getElementById('appid').value
  const gl = document.getElementById('gl').value
  const hl = document.getElementById('hl').value
  const label = document.getElementById('label').value
  const message = document.getElementById('message').value

  if (appid.indexOf('?') !== -1) {
    appid = appid.split('id=')[1].split('&')[0]
  }

  const jsonurl = `${base}i=${encodeURIComponent(appid)}&gl=${encodeURIComponent(gl)}&hl=${encodeURIComponent(hl)}&l=${encodeURIComponent(label).replace('%24', '$')}&m=${encodeURIComponent(message).replace('%24', '$')}`

  return jsonurl
}
var lastTimeout
function updateUrls () {
  window.clearTimeout(lastTimeout)
  const jsonurl = composeJsonUrl()
  const shieldurl = document.getElementById('baseendpoint').value + 'url=' + encodeURIComponent(jsonurl)
  const shieldurlstyled = document.getElementById('baseendpoint').value + 'color=green&logo=google-play&logoColor=green&url=' + encodeURIComponent(jsonurl)
  document.getElementById('jsonurl').value = jsonurl
  document.getElementById('shieldurl').value = shieldurl
  document.getElementById('shieldurlstyled').value = shieldurlstyled

  const updateDelayed = function() {
    updateImages()
    fetch(jsonurl).then(response => response.text()).then(function(text) {
      document.getElementById('json').innerHTML = text
    }).catch(function(e) {
      document.getElementById('json').innerHTML = 'Could not load JSON:\n' + e
    })
  }
  lastTimeout = window.setTimeout(updateDelayed, 1000)
}

function updateImages () {
  document.getElementById('shield').src = document.getElementById('shieldurl').value
  document.getElementById('shieldstyled').src = document.getElementById('shieldurlstyled').value
}


const copyMap = {
 'Copy Badge URL': '$url',
 'Copy Markdown': '![Custom badge]($url)',
 'Copy reStructuredText': '.. image:: $url   :alt: Custom badge',
 'Copy AsciiDoc': 'image:$url[Custom badge]',
 'Copy HTML': '<img alt="Custom badge" src="$url">',
}

function copyMenu (ev) {
  ev.stopPropagation()
  const div = document.getElementById('copymenu')
  const ul = div.querySelector('ul')
  const copy = function () {
    const self = this
    const textarea = document.getElementById(div.dataset.textareaid)
    const url = textarea.value
    const text = copyMap[self.textContent].replace(/\$url/g, url)
    textarea.value = text
    textarea.focus()
    textarea.select()
    const r = document.execCommand('copy')
    if (r) {
      textarea.value = url
      div.parentNode.parentNode.querySelector('.status').innerHTML = ' - Copied!'
    } else {
      div.parentNode.parentNode.querySelector('.status').innerHTML = ' - Clipboard not supported on this browser!'
    }
    window.setTimeout(() => div.parentNode.parentNode.querySelector('.status').innerHTML = '', 2000)
  }
  if (!('textareaid' in div.dataset)) {
    // Init
    for(let key in copyMap) {
      const li = ul.appendChild(document.createElement('li'))
      li.appendChild(document.createTextNode(key))
      li.addEventListener('click', copy)
    }
  }
  div.dataset.textareaid = this.parentNode.querySelector('textarea').id
  div.style.display = 'block'
  this.parentNode.appendChild(div)
}

function toggleMobile () {
  const isMobile = navigator.userAgent.match(/mobile/i)
  if (isMobile) {
    document.querySelectorAll('.desktop').forEach(function (e) { e.style.display = 'none' })
    document.querySelectorAll('.mobile').forEach(function (e) { e.style.display = '' })
    document.getElementById('jsonurl').size = document.getElementById('jsonurl').dataset.mobileSize
    document.querySelectorAll('.controls tr').forEach(function (tr) {
      const tds = tr.querySelectorAll('td')
      if (tds.length > 1) {
        tr.parentNode.insertBefore(tr.cloneNode(), tr).appendChild(tds[0])
      }
    })
  } else {
    document.querySelectorAll('.desktop').forEach(function (e) { e.style.display = '' })
    document.querySelectorAll('.mobile').forEach(function (e) { e.style.display = 'none' })
    document.getElementById('jsonurl').size = document.getElementById('jsonurl').dataset.desktopSize
  }
}
