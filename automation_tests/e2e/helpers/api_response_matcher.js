'use strict';

function isApiResponse(response, method, pathname, query = {}) {
  try {
    const url = new URL(response.url());
    return response.request().method() === method
      && url.pathname.endsWith(pathname)
      && Object.entries(query).every(
        ([key, value]) => url.searchParams.get(key) === String(value)
      );
  } catch {
    return false;
  }
}

function isGetResponse(response, pathname, query = {}) {
  return isApiResponse(response, 'GET', pathname, query);
}

module.exports = {
  isApiResponse,
  isGetResponse
};
