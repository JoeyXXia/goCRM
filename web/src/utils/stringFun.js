/* eslint-disable */
/** @param {string} str */
export const toUpperCase = (str) => {
  if (str[0]) {
    return str.replace(str[0], str[0].toUpperCase())
  } else {
    return ''
  }
}

/** @param {string} str */
export const toLowerCase = (str) => {
  if (str[0]) {
    return str.replace(str[0], str[0].toLowerCase())
  } else {
    return ''
  }
}

// 驼峰转换下划线
/** @param {string} str */
export const toSQLLine = (str) => {
  if (str === 'ID') return 'ID'
  return str.replace(/([A-Z])/g, '_$1').toLowerCase()
}

// 下划线转换驼峰
/** @param {string} name */
export const toHump = (name) => {
  return name.replace(/\_(\w)/g, function (all, letter) {
    return letter.toUpperCase()
  })
}
