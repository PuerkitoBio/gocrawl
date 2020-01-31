# gocrawl [![GoDoc](https://godoc.org/github.com/PuerkitoBio/gocrawl?status.png)](http://godoc.org/github.com/PuerkitoBio/gocrawl) [![build status](https://secure.travis-ci.org/PuerkitoBio/gocrawl.png)](http://travis-ci.org/PuerkitoBio/gocrawl)

gocrawl은 Go로 만든 가볍고 Concurrent한 웹 크롤러 입니다.

더 자연스러운 Go 스타일로 작성된 더 간단하고 유연한 웹 크롤러를 보고싶다면, gocrawl 의 경험이 기반이 된 패키지인 [fetchbot](https://github.com/PuerkitoBio/fetchbot) 를 참고.

## 특징 

*    방문, 검사, 쿼리할 URL 에 대한 전체 제어(미리 초기화된 [goquery][] 사용)
*    호스트당 크롤 지연 가능
*    robots.txt 규칙을 잘 따름 ([robotstxt.go][robots] 라이브러리 사용)
*    goroutines을 이용한 동시 실행
*    로깅
*    방문, URL필터링 등을 커스터마이징 할 수 있음

## 설치 방법과 의존성

gocrawl은 아래 라이브러리들이 필요합니다. :

*    [goquery][]
*    [purell][]
*    [robotstxt.go][robots]

 `golang.org/x/net/html`에 간접적인 종속성이 있어 Go1.1+ 를 요구한다. 설치:

`go get github.com/PuerkitoBio/gocrawl`

이전 버전으로 설치하려면  `$GOPATH/src/github.com/PuerkitoBio/gocrawl/` 디렉토리에 `git clone https://github.com/PuerkitoBio/gocrawl` 을 하고, (예를 들어)`git checkout v0.3.2` 을 실행하여 특정 버전을 체크아웃 하고, `go install`을 통해 Go 패키지를 빌드하고 설치한다.

## 변경 로그

*    **2019-07-22** : goquery 를 위해 미리 컴파일된 matcher 사용 (@mikefaraponov 지원). Tag v1.0.1.
*    **2016-11-20** : 로그 메세지가 URL들을 출력하도록 수정 (@oherych 지원). Tag as v1.0.0.
*    **2016-05-24** : 원래 URL 에 리다이렉션을 위해 `*URLContext.SourceURL()` 과 `*URLContext.NormalizedSourceURL()` 설정 (see [#55][i55]). 깃허브 유저 [@tmatsuo][tmatsuo] 지원.
*    **2016-02-24** : 리퀘스트 만들기 위해 항상 `Options.UserAgent` 사용, robots.txt 정책 적용 시에만 `Options.RobotUserAgent` 사용. 코드를 좀 더 나은 godoc 문서로 보냄.
*    **2014-11-06** : net/html 의 import 경로를 golang.org/x/net/html 로 변경 (https://groups.google.com/forum/#!topic/golang-nuts/eD8dh3T9yyA 참고).
*    **v0.4.1** : go-query가 go-getable 이기 때문에, go-getable.
*    **v0.4.0** : **주요 변화** 주요 리팩터와 API 변경:
    * `Extender` 인터페이스 기능의 첫 번째 인수로 표준화가 잘 되어있지 않은 `*url.URL` 포인터 대신 URL의 컨텍스트에서 호출되는 `*URLContext` 구조 사용.
    * `EnqueueOrigin` 열거 플래그 제거. gocrawl 에도 사용되지 않았고, URL과 연관된 일종의 상태여서, 이 기능은 일반화 되었다.
    * 각 URL 에 대한 `State` 추가하여 크롤링 과정이 임의 데이터를 지정된 URL 과 연결할 수 있도록 함 (예, 데이터베이스의 레코드 ID). [issue #14][i14] 수정.
    * 에러의 보다 관용적인 사용 (`ErrXxx` 변수, `Run()` 에러 반환, `EndReason` enum 필요에 의한 제거).
    * 많이 단순화된 `Filter()` 기능.이제 방문여부만 알려주는 `bool` 만 반환. 헤드 요청 오버라이드 기능은 `*URLContext` 구조에 의해 제공되며, 어디에서나 설정할 수 있다. 우선순위 특성은 구현되지 않았고 반환 값에서 제거되었으며, 구현될 경우`*URLContext` 구조를 통해서도 가능할 것이다.
    * `Start`, `Run`, `Visit`, `Visited` 와 `EnqueueChan` 모두 URL 데이터의 빈 인터페이스 유형으로 구현. 이것은 컴파일 타임 체크에는 불편하지만, 주 기능에 대해 더 많은 유연성을 제공한다. 상태가 필요하지 않은 경우에도 항상 `map[string]interface{}` 유형을 강제하는 대신, gocrawl은 [various types](#types) 지원.
    * 다른 내적 변화들, 더 나은 테스트
*    **v0.3,2** : 크롤 지연을 기다릴 때 높은 CPU 사용량 수정
*    **v0.3.1** : 기본 `Fetch()` 구현에 사용되는 `HttpClient` 변수 내보내기 ([issue #9][i9] 참고).
*    **v0.3.0** : **행동 변화** 정규화된 URL로 필터 완료, 원래 정규화되지 않은 URL로 가져오기([issue #10][i10] 참고).
*    **v0.2.0** : **주요 변화** extension/hooks 재작업.
*    **v0.1.0** : 초기 릴리즈.

## Example

`example_test.go` 에서:

```Go
// "a"로 시작하는 루트 및 경로만 열거
var rxOk = regexp.MustCompile(`http://duckduckgo\.com(/a.*)?$`)

// gocrawl 제공 기본 값을 기반으로, Extender 구현 
// 모든 메소드들을 오버라이드 하고싶지 않기 때문
type ExampleExtender struct {
    gocrawl.DefaultExtender // Visit 과 Filter를 제외한 모든 메소드의 기본 구현 사용 
}

// Visit 오버라이드
func (x *ExampleExtender) Visit(ctx *gocrawl.URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
    // 데이터 조작 위해 goquery 문서나 res.Body 사용
    // ...

    //  nil 과 true 반환 - gocrawl 이 링크 찾도록
    return nil, true
}

// Filter 오버라이드.
func (x *ExampleExtender) Filter(ctx *gocrawl.URLContext, isVisited bool) bool {
    return !isVisited && rxOk.MatchString(ctx.NormalizedURL().String())
}

func ExampleCrawl() {
    // 사용자 지정 옵션 설정 
    opts := gocrawl.NewOptions(new(ExampleExtender))

    // 항상 로봇 이름을 가장 많이 찾도록 설정해야 한다. 
    // 특정 규칙 robots.txt 에서 가능.
    opts.RobotUserAgent = "Example"
    // 요청 시 사용되는 사용자-에이전트 문자열에 반영
    // 사이트 소유자가 문제가 있을 경우 연락할 수 있도록 링크와 함께 
    opts.UserAgent = "Mozilla/5.0 (compatible; Example/1.0; +http://example.com)"

    opts.CrawlDelay = 1 * time.Second
    opts.LogFlags = gocrawl.LogAll

    // 테스트할 때 ddgo 와 잘 동작!
    opts.MaxVisits = 2

    // 크롤러 생성하고 duckduckgo 에서 시작 
    c := gocrawl.NewCrawlerWithOptions(opts)
    c.Run("https://duckduckgo.com/")

    // 출력 전 "x" 제거 : 예시 활성화 위해 (go test에서 실행 예정)

    // xOutput: 임의의 로그 출력 확인 실패 
}
```

## API

Gocrawl 은 캐싱, 지속성 및 종근성 탐지 논리로 본격적인 인덱싱 기계를 구축하거나 빠르고 쉽게 크롤링 할 수 있는 것과 같은 기본적인 엔진을 제공하는 최소화 웹 크롤러("slim" 태그를 약 1000 sloc 으로 인식) Gocrawl 자체는 페이지의 교착성을 감지하려고 시도하지 않으며 캐싱 메커니즘을 구현하지도 않는다. URL이 처리되도록 입력되면 URL을 가져오도록 요청한다. (robots.txt 에 의해 허용된다면 - "polite" 태그). 그리고 처리할 URL들 사이에 우선순위가 정해져 있지 않고, 어떤 시점에서 모든 열거된 URL들을 방문해야 하며, 그것들이 중요하지 않은 순서는 중요하지 않다고 가정한다.

그러나, 이것은 많은 [hooks and customizations](#hc)을 제공한다. 모든 것을 다 하려고 그것을 할 수 있는 방법을 강요하는 대신에, 그것은 그것을 조작하고 누구의 필요에 맞게 조정할 수 있는 방법을 제공한다. 

항상 그렇듯, 완전한 godoc 참조는 [여기][godoc]서 찾을 수 있다. 

### 설계 구조

gocrawl 의 주요 사용 사례는 `robots.txt`의 제약 사항을 존중하고 지정된 호스트에 요청 사이의 *good web citizen* 크롤 지연을 적용하면서 일부 웹페이지를 크롤링하는 것이다. 다음과 같은 설계 결정사항을 따름:

* **각 호스트는 자체 작업자를 생성 (goroutine)** : 이는 robots.txt 데이터를 먼저 읽어야 하기 때문에 타당하고, 각 패치 사이의 지연으로 한 번에 하나의 요청을 순차적으로 진행한다. 호스트 간에는 제약이 없으므로, 각각의 분리된 작업자는 독립적으로 크롤할 수 있다. 
* **방문자 기능은 goroutine 작업에서 호출된다** : 다시 말하지만, 크롤 지연이 문서를 분석하는 데 필요한 시간보다 클 가능성이 높기 때문에, 이 과정은 대개 성능에 불이익을 주지 않는다.
* **크롤 지연이 없는 엣지 케이스가 지원되지만 최적화되지 않음** :지연 없이 크롤이 필요할 때 드물지만 가능한 경우  (e.g.: 자신의 서버 또는 사용량이 많은 시간 밖에 허용된 경우, 등등.), gocrawl은 지연이 거의 없지만, 최적화를 제공하지는 않는다. 그것은, 코드에 방문자 기능이 작업자로부터 분리되거나 동일한 호스트에서 동시에 복수의 작업자를 시작할 수 있는  "특별 경로"가 없다는 것이다.(사실, 이 경우가 당신의 유일한 사용 사례라면, 나는 라이브러리를 전혀 사용하지 않는 것을 추천하고 싶다. - 왜냐하면 그것에 가치가 거의 없기 때문이다-, Go의 표준 라이브러리들을 사용하고 필요한 만큼의 gorouine들을 패치함.)
* **Extender 인터페이스는 드롭인, 완전히 캡슐화된 동작을 쓸 수 있는 수단을 제공한다** : `Extender`의 구현은 캐싱, 지속성, 다양한 패치 전략 등을 통해 코어 라이브러리를 획기적으로 향상시킬 수 있다. 그래서 `Extender.Start()` 메소드가 `Crawler.Run()` 메소드와 중복인 것, `Run`은 라이브러리로서 크롤러를 호출하는 것을 허용하고, 반면에 `Start`는 시드 URL을 선택하는 데 필요한 논리를  `Extender`에 캡슐화할 수 있게 한다. `Extender.End()`와 `Run`의 반환 값도 마찬가지다.

비록 이것이 아마도 엄청난 양의 웹 페이지를 크롤하는데 사용될 수 있지만(결국 이건 *패치, 방문, 입력, 반복적인 매스꺼움이다*!), 가장 현실적인 사용 사례(테스트된!)는 잘 알려져 있고 잘 정의된 제한된 양의 시드 버킷에 기초해야 한다. 분산 크롤링은 이 합리적인 사용법을 넘어서야 한다면, 네 친구야.

### 크롤러

크롤러 형식은 전체 실행을 제어한다. 이것은 작업자를 goroutine으로 만들고 URL 대기열을 관리한다. 두 가지 도움 생성자가 있다:

*    **NewCrawler(Extender)** : 지정된 `Extender` 객체를 사용하여 크롤러를 생성.
*    **NewCrawlerWithOptions(*Options)** : `*Options` 인스턴스가 미리 초기화된 크롤러 생성.

단 하나의 공적인 기능은 다양한 방법으로 표현할 수 있는 시드 인수를 갖는 `Run(seeds interface{}) error`다. 더 이상 방문 대기 중인 URL이 없거나 `Options.MaxVisit`수가 도달할 때 종료된다. 이 설정이 크롤링을 멈추게 한다면 `ErrMaxVisits`라는 에러를 반환한다.

<a name="types" />
시드를 통과시키는 데 사용할 수 있는 다양한 형식은 다음과 같다. (`Extender.Start(interface{}) interface{}`, `Extender.Visit(*URLContext, *http.Response, *goquery.Document) (interface{}, bool)` 과 `Extender.Visited(*URLContext, interface{})`에 있는 빈 인터페이스에 동일한 형식이 적욛, `EnqueueChan`필드의 형식과 함께):

*    `string` : 문자열로 표현된 단일 URL
*    `[]string` : 문자열로 표현된 URL의 조각
*    `*url.URL` : 구문 분석된 URL 개체의 포인터
*    `[]*url.URL` : 구문 분석된 URL 객체에 대한 포인터의 한 조각
*    `map[string]interface{}` : (키에 대한) 문자열로 표현된 URL과 관련 상태 데이터의 맵
*    `map[*url.URL]interface{}` : URL 객체(키용) 및 관련 상태 데이터에 구문 분석된 포인터로 표현된 URL 맵

편의상, `gocrawl.S` 와 `gocrawl.U` 형식은 문자열의 맵과 URL의 맵과 동등하게 제공됨, 각각(예를 들어 코드가 `gocrawl.S{"http://site.com": "some state data"}`처럼 보일 수 있음.)

### 옵션

옵션 유형은 다음 섹션에 자세히 설명되어 있으며 기본으로 초기화된 옵션 객체과 특정 `Extender` 구현을 반환하는 단일 생성자 `NewOptions(Extender)`를 제공한다.

### 후크와 사용자 정의
<a name="hc" />

 `Options`형식은 gocrawl에 의해 제공된 후크와 사용자 정의를 제공한다. `Extender`를 제외한 모든 항목은 선택 사항이며 작동 기본값이 있지만 `UserAgent` 와 `RobotUserAgent` 옵션은 프로젝트에 대한 사용자 정의 값 피팅으로 설정해야 한다.

*    **UserAgent** : 페이지를 가져오는 데 사용되는 사용자-에이전트 문자열. 윈도우즈 사용자-에이전트 문자열의 Firefox 15로 기본 설정되어 있음. robot 이름과 연락처 링크를 포함하도록 변경해야 함 (예시 참고).

*    **RobotUserAgent** : robot.txt 파일에서 일치하는 정책을 찾는 데 사용되는 robot의 사용자-에이전트 문자열. `M.m`이 gocrawl의 메이저와 마이너 버전인`Googlebot (gocrawl vM.m)`에 기본 설정되어 있음. 이것은 프로젝트 이름처럼 **커스텀 값으로 항상 변경 되어야한다** (예시 참고). robot의 사용자 에이전트를 기반으로 하는 규칙-일치에 대한 자세한 정보는 [robots exclusion protocol][robprot] ([full specification as interpreted by Google here][robspec]) 참고. 사이트 소유자가 사용자에게 연락할 필요가 있는 경우 사용자 에이전트에 연락처 정보를 포함하는 것이 좋은 관행이다.

*    **MaxVisits** : 크롤을 중지하기 전에 *방문된* 최대 페이지 수. 아마도 개발 목적으로 더 유용할 것이다. 크롤러는 이 방문 횟수에 도달하면 정지 신호를 보내지만 작업자가 다른 페이지를 방문하는 과정에 있을 수 있으므로 크롤링을 멈추면 *최소* MaxVisits, 그 이상일 수 있음(최악은 `MaxVisits + number of active workers`). 최대값이 아닌 0으로 기본 설정.

*    **EnqueueChanBuffer** : 입력 채널의 버퍼 크기 (익스텐더가 크롤러에 새 URL을 임의로 입력할 수 있는 채널). 100으로 기본 설정.

*    **HostBufferFactor** : `SameHostOnly`를 `false`로 설정한 경우 작업자 맵과 통신 채널의 규모에 대한 인자. SameHostOnly가 `true`라면, 크롤러는 필요한 크기를 정확히 알고있지만 (시드 URL을 기반으로 한 다른 호스트의 수)), `false`일 때는,그 크기가 기하급수적으로 커질 수 있다. 기본적으로, 10의 인자가 사용된다 (크기는 시드 URL을 기준으로 서로 다른 호스트 수의 10배로 설정됨).

*    **CrawlDelay** : 같은 호스트로의 요청사이의 기다리는 시간. 호스트로부터 요청을 받자마자 지연 시작. 이것은 `time.Duration` 유형, 예를 들어 `5 * time.Second`로 특정지어질 수 있다. (기본값, 5초). **크롤 지연이 robots.txt 파일에 명시되어 있다면, robot의 사용자-에이전트와 일치하는 그룹에서 기본적으로 이 지연이 대신 사용됨** 크롤 지연이 `ComputeDelay` extender 기능을 구현하는 것에 의해 사용자 정의될 수 있다.

*    **WorkerIdleTTL** : 삭제하기 전에 작업자에게 허용된 유휴시간 (goroutine 정지되어있음). 10초가 기본값. 크롤 지연은 유휴 시간의 일부가 아니며, 이는 특히 작업자가 사용 가능한 시간이지만 처리할 URL이 없다.

*    **SameHostOnly** : URL을 동일한 호스트를 대상으로 하는 링크로만 입력하도록 제한, 기본적으로 true.

*    **HeadBeforeGet** : 크롤러에게 최종 GET 요청을 하기 전에 HEAD 요청(및 후속 `RequestGet()` 메소드 호출)을 발행하도록 요청한다. 이것은 기본적으로 `false`로 설정되어 있다. 아래에 설명된 `URLContext` 구조 참조.

*    **URLNormalizationFlags** : [purell][] 라이브러리를 사용하여 URL을 정규화할 때 적용할 플래그. URL은 입력되기 전에 정규화되어 `URLContext`구조의 `Extender` 메소드로 전달된다. purell, `purell.FlagsAllGreedy`에서 허용하는 가장 적극적인 정규화로 기본 설정.

*    **LogFlags** : 로깅에 대한 장황함의 레벨. (`LogError`)만으로 에러 기본 설정. 플래그 세트일 수 있음. (i.e. `LogError | LogTrace`).

*    **Extender** : `Extender` 인터페이스를 구현하는 객체. gocrawl이 제공하는 다양한 콜백 실행. `Crawler`를 생성할 때 지정해야 함. (또는 `NewCrawlerWithOptions` 생성자에게 전달할 `Options`을 생성할 때). 기본 extender가 유효한 기본 구현으로 제공됨 , `DefaultExtender`. 모든 방법에 사용자 지정이 필요하지 않은 경우 [익명 필드로 삽입하여][gotalk] 사용자 지정 익스텐더를 구현하여 사용할 수 있음 (위 예시 참고).

### 익스텐더 인터페이스

이 마지막 옵션 필드인, `익스텐더`, gocrawl을 사용하는데 있어서 매우 중요하며, 그래서 `익스텐더` 인터페이스에서 요구하는 각 콜백 기능에 대한 세부 사항은 다음과 같다.

*    **Start** : `Start(seeds interface{}) interface{}`. `Run`이 크롤러에 호출될 때 호출되며, 인자로 시드들이 `Run`로 전달된다. 실제로 시드로 사용될 데이터를 반환하므로, 이 콜백은 크롤러에 의해 처리되는 시드를 제어할 수 있다. 더 많은 정보를 위해선 [the various supported types](#types) 참고. 기본적으로, 이건 패스스루로, 인수로 수신된 데이터를 반환한다.

*    **End** : `End(err error)`. 크롤링이 끝날 때 에러나 nil 과 함께 호출된다. 똑같은 에러가 `Crawler.Run()` 함수에서도 반환된다. 기본적으로, 이 메소드는 no-op이다.

*    **Error** : `Error(err *CrawlError)`. 크롤링 에러가 일어날 때 호출된다. 에러가 크롤링 실행을 멈추지는 **않는다**. [`CrawlError`][ce] 객체는 인자로 전달된다. 이 오류 구현에는 오류가 발생한 단계를 나타내는 `Kind` 필드, 오류를 발생시킨 URL을 식별하는 `*URLContext` 필드가 포함된다. 기본적으로, 이 메소드는 no-op이다.

*    **Log** : `Log(logFlags LogFlags, msgLevel LogFlags, msg string)`. 로깅 기능 기본적으로, 표준 오류 (Stderr)로 출력하고, `LogFlags` 옵션에 포함된 레벨의 메세지만 출력한다. 사용자 정의 `Log()` 메소드를 구현한다면, 요청된 상세성 수준에 따라 메시지를 고려해야 하는지 여부를 확인하는 것은 귀하에게 달려 있는데 (i.e. `if logFlags&msgLevel == msgLevel ...`), 이 메소드는 항상 모든 메시지에 대해 호출되기 때문이다.

*    **ComputeDelay** : `ComputeDelay(host string, di *DelayInfo, lastFetch *FetchInfo) time.Duration`. URL을 요청하기 전에 작업자에 의해 호출됨. 인자들은 호스트 이름(`*url.URL.Host`의 정규화된 형식), 크롤 지연 정보(옵션 구조, robots.txt, 마지막으로 사용된 지연으로부터의 지연 포함), 마지막 패치 정보인데, 호스트의 현재 응답성에 적응할 수 있도록 한다. 그것은 사용할 지연을 반환한다.

나머지 확장 기능은 모두 주어진 URL의 문맥에서 호출되므로, 이들의 첫 번째 인수는 항상 `URLContext` 구조의 포인터가 된다. 따라서 이러한 메소드를 문서화하기 전에 모든 `URLContext`필드 및 메소드에 대한 설명을 참조:

* `HeadBeforeGet bool` : 이 필드는 크롤러의 `Options` 구조에서 글로벌 설정으로 초기화 된다. HEAD 요청 여부를 결정하는 `Fetch`를 호출하기 전에 언제든지 재지정할 수 있다.
* `State interface{}` : 이 필드에는 URL과 관련된 임의 상태 데이터가 저장되어 있다. `nil`일 수도 있고 어떤 종류의 값일 수도 있다. 
* `URL() *url.URL` : 구문 분석된 URL을 정규화되지 않은 형식으로 반환하는 getter 메소드.
* `NormalizedURL() *url.URL` : 구문 분석된 URL을 정규화된 형식으로 반환하는 getter 메서드.
* `SourceURL() *url.URL` : 소스 URL을 정규화되지 않은 형식으로 반환하는 getter 메서드. `EnqueueChan`을 통해 입력되는 URL이나 시드의 경우 `nil`이 될 수 있다.
* `NormalizedSourceURL() *url.URL` : 소스 URL을 정규화된 형식으로 반환하는 getter 메서드. `EnqueueChan`을 통해 입력되는 URL이나 시드의 경우 `nil`이 될 수 있다.
* `IsRobotsURL() bool` : 현재 URL이 robots.txt URL인지 여부를 표시한다.

다른 `Extender` 기능들이 존재한다:

*    **Fetch** : `Fetch(ctx *URLContext, userAgent string, headRequest bool) (*http.Response, error)`. 작업자가 URL을 요청하기 위해 호출함. 작업자가 리디렉션 URL을 입력할 수 있도록 특수 오류 (`ErrEnqueueRedirect`)를 반환하는 것 대신 수정사항을 따르지 않고 페이지를 가져오기위해 `DefaultExtender.Fetch()` 구현에서는 공용 `HttpClient` 변수 (맞춤 `http.Client`)를 사용한다. 이것은 크롤링 과정에 의해 패치된 모든 URL의 `Filter()`에 의한 화이트리스트를 시행한다. `headRequest`가 `true`일 경우 GET 대신 HEAD 요청이 이루어진다. gocrawl v0.3의 경우, 기본 `Fetch` 구현은 비정규화된 URL을 사용한다.

     내부적으로, gocrawl은 http.클라이언트의 `CheckRedirect()` 기능 필드를 robots.txt URL만을 따르는 사용자 정의 구현으로 설정한다. (왜냐면 robots.txt에 리디렉션은 여전히 사이트 소유자가 이 호스트에 대해 이 규칙을 사용하기를 원한다는 것을 의미한다.). 작업자는 `ErrEnqueueRedirect` 오류를 알고 있으므로, robots.txt URL이 아닌 경우 리디렉션을 요청하면, `CheckRedirect()`가 이 오류를 반환하고, 작업자는 이를 인식하고 리디렉션-to URL을 입력하여 현재 URL의 처리를 중지한다. 동일한 논리에 근거해 맞춤형 `Fetch()` 구현을 제공할 수 있다.  `ErrEnqueueRedirect` 오류를 반환하는 `CheckRedirect()` 구현은 다음과 같이 동작한다 - 그것은, 작업자는 이 오류를 감지할 것이며, redirect-to URL을 입력할 것이다. 자세한 내용은 소스 파일 ext.go 및 worker.go를 참조하십시오.

    `HttpClient`변수는 공개되어 `CheckRedirect()` 함수 또는 다른 `Transport` 객체를 사용하도록 사용자 지정할 수 있다. 이 사용자 지정은 크롤러를 시작하기 전에 수행해야 한다. 기본 `Fetch()` 구현에 의해 사용되거나, 필요한 경우 사용자 정의 `Fetch()`에서도 사용될 수 있다. 이 클라이언트는 응용 프로그램의 모든 크롤러가 공유한다는 점에 유의하십시오. 동일한 애플리케이션에서 크롤러마다 다른 http 클라이언트가 필요할 경우 개인 `http.Client`을 사용한 사용자 정의 `Fetch()`가 제공되어야한다.

*    **RequestGet** : `RequestGet(ctx *URLContext, headRes *http.Response) bool`. 크롤러가 HEAD 요청의 응답을 바탕으로 GET 요청을 진행해야 하는지 여부를 표시한다.이 메소드는 HEAD가 요청된 경우에만 호출된다. (`*URLContext.HeadBeforeGet` 필드 기반). 기본 구현은 HEAD 응답 상태 코드가 2xx 라면`true` 반환.

*    **RequestRobots** : `RequestRobots(ctx *URLContext, robotAgent string) (data []byte, request bool)`. robots.txt URL이 패치되어야 하는지 묻는다. 두 번째 값으로 `false`가 반환되면, `data` 값은 robots.txt의 캐쉬 컨텐츠로 여겨지고, 그렇게 쓰여진다 (비어 있을 경우, robots.txt가 없었던 것 처럼 한다). `DefaultExtender.RequestRobots` 은 `nil, true` 반환.

*    **FetchedRobots** : `FetchedRobots(ctx *URLContext, res *http.Response)`. robots.txt URL이 호스트로 부터 패치될 때 호출되고, 콘첸츠를 캐쉬하여 향후 `RequestRobots()` 호출로 다시 제공할 수 있다. 기본적으로 이건 no-op.

*    **Filter** : `Filter(ctx *URLContext, isVisited bool) bool`. 방문에 대한 URL의 입력 여부를 결정할 때 호출됨. `*URLContext`와 크롤링 실행에서 이 URL이 이미 방문되었는지 여부를 표시하는 `bool` "is visited" 플래그를 받는다. gocrawl이 URL을 방문 (`true`)하거나나 무시 (`false`)하도록 하는`bool` flag를 반환한다. 함수가 방문하는 URL을 입력하기 위해 `true`를 반환하더라도, 정규화된 형태의 URL은 다음 규칙을 준수해야 한다:

1. 절대 URL 이어야 한다.
2. `http/https` 형식 가져야 한다.
3. `SameHostOnly` 플래그가 설정되어 있을 경우 같은 호스트를 가져야 한다.

    URL을 아직 방문하지 않은 경우 `DefaultExtender.Filter`은 `true`를 반환 (*visited* 플래그 URL의 정규화된 버전을 기반으로 한다.), 그렇지 않으면 false.

*    **Enqueued** : `Enqueued(ctx *URLContext)`. 크롤러에 의해 URL이 입력되었을 때 호출된다. 입력된 URL은 robot.txt 정책에 의해 허용되지 않을 수 있다. 그래서 결국 패치되지 못할 수 있다. 기본적으로 이 방법은 금지되어 있다.

*    **Visit** : `Visit(ctx *URLContext, res *http.Response, doc *goquery.Document) (harvested interface{}, findLinks bool)`. URL 방문할 때 호출. 사용 가능한`*goquery.Document`객체와 함께 URL 컨텍스트인 `*http.Response` response 객체를 수신 (아니면 `nil` 응답 본문을 구문 분석할 수 없는 경우). 프로세스에 링크와 gocrawl이 직접 링크를 찾아야 하는지 여부를 나타내는`bool`을 반환. ( 가능한 형식 확인하려면 [above](#types) 참고). 플래그가 `true`면, `harvested` 반환 값은 무시되고 입력할 링크를 위해 gocrawl은 goquery 문서를 검색. `false`면, `harvested` 데이터는 입력된다. `DefaultExtender.Visit` 구현은 `nil, true`을 반환하여 방문 페이지의 링크를 자동으로 찾아서 처리하도록 한다.

*    **Visited** : `Visited(ctx *URLContext, harvested interface{})`. 페이지를 방문한 후 호출됨. 방문 중에 발견된 URL 컨텍스트 및 URL들은 인자로 전달됨.(`Visit` 기능이나 gocrawl 에 의해). 기본적으로 이 함수는 비어 있습니다.

*    **Disallowed** : `Disallowed(ctx *URLContext)`. 입력된 URL이 robots.txt 정책에 의해 거부될 때 호출된다. 기본적으로 이 함수는 비어있습니다.

마지막으로, 관례상, 매우 구체적인 유형의 `chan<- interface{}`를 가진 `EnqueueChan`이라는 필드가 존재하여 `Extender` 인스턴스에서 접속할 수 있는 경우, 이 필드는 [the expected types](#types)을 URL을 입력하기 위한 데이터로 받아들이는 입력 채널로 설정된다. 그러면 이 데이터는 마치 방문에서 수확한 것처럼 크롤러에 의해 처리될 것이다. `Filter()`에 대한 호출을 촉발하고, 하영될 경우 패치되고 방문하게 된다.

`DefaultExtender` 구조는 유효한 `EnqueueChan` 필드를 가지고 있어, 사용자 정의 Extender 구조에서 익명 필드로 내장되면 이 구조는 자동으로`EnqueueChan` 기능을 얻는다.

이 채널은 크롤링 과정에 의해 처리되지 않은 URL을 임의로 입력하는 데 유용할 수 있다. 예를 들어 URL에서 서버 오류 (상태 코드 5xx)가 발생할 경우 `Error()` extender 함수에 다시 입력하여 다른 패치를 시도할 수 있다.

## 수고해주신 분들

* Richard Penman
* Dmitry Bondarenko
* Markus Sonderegger
* @lionking6792

## License

[BSD 3-Clause license][bsd].

[bsd]: http://opensource.org/licenses/BSD-3-Clause
[goquery]: https://github.com/PuerkitoBio/goquery
[robots]: https://github.com/temoto/robotstxt.go
[purell]: https://github.com/PuerkitoBio/purell
[robprot]: http://www.robotstxt.org/robotstxt.html
[robspec]: https://developers.google.com/webmasters/control-crawl-index/docs/robots_txt
[godoc]: http://godoc.org/github.com/PuerkitoBio/gocrawl
[er]: http://godoc.org/github.com/PuerkitoBio/gocrawl#EndReason
[ce]: http://godoc.org/github.com/PuerkitoBio/gocrawl#CrawlError
[gotalk]: http://talks.golang.org/2012/chat.slide#33
[i10]: https://github.com/PuerkitoBio/gocrawl/issues/10
[i9]: https://github.com/PuerkitoBio/gocrawl/issues/9
[i14]: https://github.com/PuerkitoBio/gocrawl/issues/14
[i55]: https://github.com/PuerkitoBio/gocrawl/issues/55
[tmatsuo]: https://github.com/tmatsuo
