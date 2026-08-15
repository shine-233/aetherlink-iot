const { expect } = require('chai');
const {
  isApiResponse,
  isGetResponse
} = require('../e2e/helpers/api_response_matcher');

function response(url, method = 'GET') {
  return {
    url: () => url,
    request: () => ({ method: () => method })
  };
}

describe('E2E API response matcher contract', function () {
  it('matches the method and pathname suffix', function () {
    const candidate = response('http://localhost:9999/proxy-default/scene/detail/42');

    expect(isGetResponse(candidate, '/scene/detail/42')).to.equal(true);
    expect(isGetResponse(candidate, '/scene/detail/41')).to.equal(false);
    expect(isApiResponse(candidate, 'POST', '/scene/detail/42')).to.equal(false);
  });

  it('matches every query value using its string representation', function () {
    const candidate = response('http://localhost/api/v1/ui_elements?page=1&page_size=10&enabled=false');

    expect(isGetResponse(candidate, '/ui_elements', {
      page: 1,
      page_size: 10,
      enabled: false
    })).to.equal(true);
    expect(isGetResponse(candidate, '/ui_elements', { page: 2 })).to.equal(false);
    expect(isGetResponse(candidate, '/ui_elements', { missing: 'value' })).to.equal(false);
  });

  it('returns false when the response contract or URL is invalid', function () {
    expect(isGetResponse(response('not a URL'), '/scene')).to.equal(false);
    expect(isGetResponse({ url: () => 'http://localhost/scene' }, '/scene')).to.equal(false);
    expect(isGetResponse(null, '/scene')).to.equal(false);
  });
});
