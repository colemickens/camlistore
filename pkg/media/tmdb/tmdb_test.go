package tmdb

// This is also kind of a test for mediautil basically

import (
	"log"
	"os"
	"testing"

	mediautil "camlistore.org/pkg/media/util"
)

func getTmdbApi(t *testing.T) *TmdbApi {
	tmdbApi, err := NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil) // insert my test key, whatever or pipe it in?
	if err != nil {
		t.Fatal(err)
	}
	log.Println(tmdbApi.Config)
	return tmdbApi
}

func TestTmdbConfig(t *testing.T) {
	getTmdbApi(t)
}

func TestTmdbSearchMovies(t *testing.T) {

	// extract checkStr() checkInt() into a helper pkg, tired of redoing them repeatedly
}

func TestLookupMovies(t *testing.T) {
	dir := "/media/data/Media/movies"
	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	expectedResults := map[string]struct {
		Title  string
		Year   int
		TmdbId int
	}{
		"lotr.1.the.fellowship.of.the.ring.1080p.mkv": {"lord of the rings 1 the fellowship of the ring", -1, 0},
		"lotr.2.the.two.towers.1080p.mkv":             {"lord of the rings 2 two towers", -1, 0},
		"lotr.3.the.return.of.the.king.1080p.mkv":     {"lord of the rings 3 the return of the king", -1, 0},

		"Star.Wars.Episode.1.the.phantom.menace.1999.720p.mkv":                             {"Star Wars Episode 1 the phantom menace", 1999, 0},
		"Star.Wars.Episode.2.Attack.of.the.Clones.2002.720p.BluRay.nHD.x264-NhaNc3.mkv":    {"Star Wars Episode 2 Attack of the Clones", 2002, 0},
		"Star.Wars.Episode.3.Revenge.of.the.Sith.2005.720p.BluRay.nHD.x264-NhaNc3.mkv":     {"Star Wars Episode 3 Revenge of the Sith", 2005, 0},
		"Star.Wars.Episode.4.A.New.Hope.1977.720p.BluRay.nHD.x264-NhaNc3.mkv":              {"Star Wars Episode 4 A New Hope", 1977, 0},
		"Star.Wars.Episode.5.The.Empire.Strikes.Back.1980.720p.BluRay.nHD.x264-NhaNc3.mkv": {"Star Wars Episode 5 The Empire Strikes Back", 1980, 0},
		"Star.Wars.Episode.6.Return.of.the.Jedi.1983.720p.BluRay.nHD.x264-NhaNc3.mkv":      {"Star Wars Episode 6 Return of the Jedi", 1983, 0},

		"mission.impossible.1.1996.mp4":                    {"mission impossible 1", 1996, 954},
		"mission.impossible.2.2000.mp4":                    {"mission impossible 2", 2000, 955},
		"mission.impossible.3.2006.mp4":                    {"mission impossible 3", 2006, 956},
		"mission.impossible.ghost.protocol.2011.1080p.mkv": {"mission impossible ghost protocol", 2011, 56292},

		"Indiana Jones 1 Raiders Of The Lost Ark 1981.mp4":      {"Indiana Jones 1 Raiders Of The Lost Ark", 1981, 85},
		"Indiana Jones 2 Temple Of Doom 1984.mp4":               {"Indiana Jones 2 Temple Of Doom", 1984, 87},
		"Indiana Jones 3 Last Crusade 1989.mp4":                 {"Indiana Jones 3 Last Crusade", 1989, 89},
		"Indiana Jones 4 Kingdom Of The Crystal Skull 2008.mp4": {"Indiana Jones 4 Kingdom Of The Crystal Skull", 2008, 217},

		"A Beautiful Mind.mp4":                                    {"A Beautiful Mind", -1, 453},
		"A Single Man.mkv":                                        {"A Single Man", -1, 34653},
		"Adaptation.avi":                                          {"Adaptation", -1, 2757},
		"American Beauty.avi":                                     {"American Beauty", -1, 0},
		"American Psycho.avi":                                     {"American Psycho", -1, 1359},
		"American.Gangster.720p.mkv":                              {"American Gangster", -1, 0},
		"Avatar.mkv":                                              {"Avatar", -1, 19995},
		"bad.teacher.720p.mkv":                                    {"bad teacher", -1, 0},
		"Black Swan.720p.avi":                                     {"Black Swan", -1, 0},
		"Blade Runner.dvdrip.avi":                                 {"Blade Runner", -1, 0},
		"brazil.dvdrip.avi":                                       {"brazil", -1, 0},
		"Brick.dvdrip.avi":                                        {"Brick", -1, 0},
		"Brokeback Mountain.avi":                                  {"Brokeback Mountain", -1, 142},
		"captain.america.the.first.avenger.720p.mkv":              {"captain america the first avenger", -1, 0},
		"Children of Men.avi":                                     {"Children of Men", -1, 9693},
		"Death at a Funeral.avi":                                  {"Death at a Funeral", -1, 2196},
		"Deja.Vu.720p.mkv":                                        {"Deja Vu", -1, 0},
		"District 9.mkv":                                          {"District 9", -1, 17654},
		"Donnie Darko.avi":                                        {"Donnie Darko", -1, 141},
		"Due Date.mp4":                                            {"Due Date", -1, 41733},
		"Eternal Sunshine of the Spotless Mind.mkv":               {"Eternal Sunshine of the Spotless Mind", -1, 38},
		"Exit Through the Gift Shop.mp4":                          {"Exit Through the Gift Shop", -1, 39452},
		"Fair Game.1080p.mkv":                                     {"Fair Game", -1, 0},
		"ferris.buellers.day.off.1080p.mkv":                       {"ferris buellers day off", -1, 0},
		"fight.club.mp4":                                          {"fight club", -1, 550},
		"harry.potter.7.part.1.720p.mkv":                          {"harry potter 7 part 1", -1, 0},
		"hereafter.720p.mkv":                                      {"hereafter", -1, 0},
		"horrible.bosses.720p.mkv":                                {"horrible bosses", -1, 0},
		"inception.1080p.mkv":                                     {"inception", -1, 0},
		"inside.job.mkv":                                          {"inside job", -1, 44639},
		"insidious.720p.avi":                                      {"insidious", -1, 0},
		"jackass.3.5.avi":                                         {"jackass 3 5", -1, 0},
		"jackass.3.mp4":                                           {"jackass 3", -1, 16290},
		"Legion.avi":                                              {"Legion", -1, 22894},
		"Little Fockers.720p.mkv":                                 {"Little Fockers", -1, 0},
		"Lost in Space.avi":                                       {"Lost in Space", -1, 2157},
		"Milk.avi":                                                {"Milk", -1, 10139},
		"Minority Report.mp4":                                     {"Minority Report", -1, 0},
		"Mysterious Skin.dvd.avi":                                 {"Mysterious Skin", -1, 0},
		"Next 3 Days.mkv":                                         {"Next 3 Days ", -1, 43539},
		"Pandorum.mkv":                                            {"Pandorum", -1, 19898},
		"prayers.for.bobby.avi":                                   {"prayers for bobby", -1, 21634},
		"Religulous.avi":                                          {"religulous", -1, 13007},
		"rise.of.the.planet.of.the.apes.720p.mkv":                 {"rise of the planet of the apes", -1, 0},
		"Scott Pilgrim Vs The World.720p.mkv":                     {"Scott Pilgrim Vs The World", -1, 0},
		"serenity.720p.mkv":                                       {"serenity", -1, 0},
		"Solaris.avi":                                             {"Solaris", -1, 2103},
		"Surrogates.mkv":                                          {"Surrogates", -1, 19959},
		"The Ghost Writer.avi":                                    {"The Ghost Writer", -1, 11439},
		"The Green Hornet.720p.mkv":                               {"The Green Hornet", -1, 0},
		"The Kings Speech.1080p.mkv":                              {"The Kings Speech", -1, 0},
		"The Matrix Reloaded.720p.mkv":                            {"The Matrix Reloaded", -1, 0},
		"The Matrix Revolutions.720p.mkv":                         {"The Matrix Revolutions", -1, 0},
		"The Matrix.720p.mkv":                                     {"The Matrix", -1, 0},
		"The Mechanic.720p.avi":                                   {"The Mechanic", -1, 0},
		"The Orphanage.avi":                                       {"The Orphanage", -1, 6537},
		"The Other Guys.720p.mkv":                                 {"The Other Guys", -1, 0},
		"The Rules Of Attraction.avi":                             {"The Rules Of Attraction", -1, 1809},
		"The Yes Men Fix the World.avi":                           {"The Yes Men Fix the World", -1, 18489},
		"the.departed.mp4":                                        {"the departed", -1, 1422},
		"the.island.dvdrip.avi":                                   {"the island", -1, 0},
		"the.number.23.720p.mkv":                                  {"the number 23", -1, 0},
		"the.silence.of.the.lambs.720p.mkv":                       {"the silence of the lambs", -1, 0},
		"the.social.network.720p.mkv":                             {"the social network", -1, 0},
		"the.truman.show.720p.mp4":                                {"the truman show", -1, 0},
		"Tron Legacy.1080p.mkv":                                   {"Tron Legacy", -1, 0},
		"True Grit.720p.mkv":                                      {"True Grit", -1, 0},
		"Unknown.720p.mkv":                                        {"Unknown", -1, 0},
		"Unstoppable.720p.mkv":                                    {"Unstoppable", -1, 0},
		"Up.1080p.mkv":                                            {"Up", -1, 0},
		"30.minutes.or.less.720p.mkv":                             {"30 minutes or less", -1, 62206},
		"50.50.720p.mkv":                                          {"50 50", -1, 0},
		"chronicle.720p.mkv":                                      {"chronicle", -1, 0},
		"collateral.720p.mp4":                                     {"collateral", -1, 0},
		"contact.1080p.mkv":                                       {"contact", -1, 0},
		"apollo.18.720p.mkv":                                      {"apollo 18", -1, 0},
		"the.help.720p.mkv":                                       {"the help", -1, 0},
		"the.avengers.1080p.mkv":                                  {"the avengers", -1, 0},
		"get.him.to.the.greek.720p.mkv":                           {"get him to the greek", -1, 0},
		"tinker.tailor.soldier.spy.mkv":                           {"tinker tailor soldier spy", -1, 49517},
		"walk.the.line.1080p.mkv":                                 {"walk the line", -1, 0},
		"xmen.origins.wolverine.1080p.mp4":                        {"xmen origins wolverine", -1, 0},
		"harold.kumar.christmas.mkv":                              {"harold kumar christmas", -1, 55465},
		"harry.potter.7.part.2.mkv":                               {"harry potter 7 part 2", -1, 12445},
		"the.inbetweeners.movie.avi":                              {"the inbetweeners movie", -1, 6979},
		"the.prestige.mp4":                                        {"the prestige", -1, 1124},
		"the.rock.720p.mkv":                                       {"the rock", -1, 0},
		"Inglorious Basterds.mkv":                                 {"Inglorious Basterds", -1, 16869},
		"j.edgar.720p.mkv":                                        {"j edgar", -1, 0},
		"killer.elite.720p.mkv":                                   {"killer elite", -1, 0},
		"Lockout.720p.mkv":                                        {"Lockout", -1, 0},
		"outbreak.avi":                                            {"outbreak", -1, 6950},
		"perfect.sense.720p.mkv":                                  {"perfect sense", -1, 0},
		"the.bourne.supremacy.mp4":                                {"the bourne supremacy", -1, 2502},
		"the.bourne.ultimatum.mp4":                                {"the bourne ultimatum", -1, 2403},
		"the.debt.720p.mkv":                                       {"the debt", -1, 0},
		"the.fifth.element.mkv":                                   {"the fifth element", -1, 18},
		"The Town Extended Cut.mp4":                               {"The Town", -1, 0},
		"the.big.lebowski.1080p.2.mkv":                            {"the big lebowski", -1, 115},
		"silver.linings.playbook.dvdscr.mkv":                      {"silver linings playbook", -1, 0},
		"its.complicated.720p.mkv":                                {"its complicated", -1, 217316},
		"Somebody.Up.There.Likes.Me.720p.WEB-DL.AAC2.0.H.264.mkv": {"Somebody Up There Likes Me", -1, 0},
		"shutter.island.720p.mkv":                                 {"shutter island", -1, 0},
		"127.hours.720p.mkv":                                      {"127 hours", -1, 0},
		"21.jump.street.720p.mkv":                                 {"21 jump street", -1, 0},
		"Oz.The.Great.And.Powerful.720p.sparks.mkv":               {"Oz The Great And Powerful", -1, 0},
		"Samsara.DTS.1080p.mkv":                                   {"Samsara", -1, 0},
		"2001 A Space Odyssey.mkv":                                {"2001 A Space Odyssey", -1, 0},

		"this.means.war.2012.unrated.720p.mkv":                                   {"this means war", 2012, 59962},
		"limitless.2011.720p.mkv":                                                {"limitless", 2011, 51876},
		"primer.2004.720p.web-dl.mkv":                                            {"primer", 2004, 14337},
		"source.code.2011.avi":                                                   {"source code", 2011, 45612},
		"In.Time.2011.720p.mkv":                                                  {"In Time", 2011, 49530},
		"bringing.down.the.house.2003.720p.mkv":                                  {"bringing down the house", 2003, 10678},
		"battlestar.galactica.blood.and.chrome.2012.720p.bluray.x264-geckos.mkv": {"battlestar galactica blood and chrome", 2012, 156078},
		"Life.of.Pi.2012.720p.WEB-DL.DD5.1.H.264-HD4FUN.mkv":                     {"Life of Pi", 2012, 87827},
		"zero.dark.thirty.2012.720p.mkv":                                         {"zero dark thirty", 2012, 0},
		"lincoln.2012.720p.mkv":                                                  {"lincoln", 2012, 72976},
		"les.miserables.2012.720p.bluray.x264-sparks.mkv":                        {"les miserables", 2012, 82695},
		"the.hobbit.an.unexpected.journey.2012.720p.bluray.x264-sparks.mkv":      {"the hobbit an unexpected journey", 2012, 49051},
		"mr.and.mrs.smith.2005.720p.mkv":                                         {"mr and mrs smith", 2005, 787},
		"moon.2009.720p.mkv":                                                     {"moon", 2009, 17431},
		"hansel.and.gretel.2013.mkv":                                             {"hansel and gretel", 2013, 200462},
		"the.hit.list.2011.720p.mkv":                                             {"the hit list", 2011, 58626},
		"project.x.2012.extended.720p.mkv":                                       {"project x", 2012, 57214},
		"The.Adjustment.Bureau.2011.720p.BluRay.x264.DTS-CtrlHD.mkv":             {"The Adjustment Bureau", 2011, 38050},
		"drive.2012.720p.mkv":                                                    {"drive", 2012, 64690},
		"sin.city.2005.720p.mkv":                                                 {"sin city", 2005, 0},
		"melancholia.2011.720p.mkv":                                              {"melancholia", 2011, 62215},
		"monsters.2010.720p.mkv":                                                 {"monsters", 2010, 43933},
		"the.big.lebowski.1998.1080p.mkv":                                        {"the big lebowski", 1998, 115},
		"zodiac.2007.720p.mkv":                                                   {"zodiac", 2007, 1949},
		"step.brothers.unrated.2008.mkv":                                         {"step brotheres", 2008, 0},
		"lucky.number.slevin.2006.1080p.mkv":                                     {"lucky number slevin", 2006, 186},
		"role.models.unrated.2008.720p.mkv":                                      {"role models", 2008, 0},
		"dredd.2012.720p.mkv":                                                    {"dredd", 2012, 49049},
		"animal.house.1978.720.mkv":                                              {"animal house", 1978, 8469},
		"bernie.2011.limited.720p.mkv":                                           {"bernie", 2011, 92591},
		"clerks.1994.the.first.cut.1080p.mkv":                                    {"clerks", 1994, 2292},
		"clerks.ii.2006.720p.mkv":                                                {"clerks ii", 2006, 2295},
		"clerks.ii.2006.UNKNOWN.mkv":                                             {"clerks ii", 2006, 2295},
		"hot.fuzz.2007.1080p.mkv":                                                {"hot fuzz", 2007, 4638},
		"the.bourne.legacy.2012.720p.mp4":                                        {"the bourne legacy", 2012, 49040},
		"looper.2012.1080p.mkv":                                                  {"looper", 2012, 59967},
		"the.bourne.identity.2002.mp4":                                           {"the bourne identity", 2002, 0},
		"bruno.2009.720p.mkv":                                                    {"bruno", 2009, 18480},
		"dilemma.2011.720p.mkv":                                                  {"dilemma", 2011, 44564},
		"Gattaca.1997.mp4":                                                       {"Gattaca", 1997, 782},
		"eagle.eye.2008.mp4":                                                     {"eagle eye", 2008, 13027},
		"Machete.2010.1080p.mkv":                                                 {"Machete", 2010, 23631},
		"the.saint.1997.avi":                                                     {"the saint", 1997, 10003},
		"Salt.2010.1080p.mkv":                                                    {"Salt", 2010, 27576},
		"planes.trains.automobiles.1987.1080p.mkv":                               {"planes trains automobiles", 1987, 2609},
		"christmas.vacation.1989.1080p.mkv":                                      {"christmas vacation", 1989, 5825},
		"cloud.atlas.2012.720p.mkv":                                              {"cloud atlas", 2012, 83542},
		"total.recall.2012.extended.1080p.mkv":                                   {"total recall", 2012, 64635},
		"xmen.2000.1080p.mp4":                                                    {"xmen", 2000, 36657},
		"x2.2003.1080p.mp4":                                                      {"x2", 2003, 36658},
		"xmen.the.last.stand.2006.1080p.mp4":                                     {"xmen the last stand", 2006, 36668},
		"xmen.first.class.2011.720p.avi":                                         {"xmen first class", 2011, 49538},
		"paranorman.2012.720p.mkv":                                               {"paranorman", 2012, 1878},
		"the.watch.2012.720p.mkv":                                                {"the watch", 2012, 80035},
		"the.campaign.2012.extended.720p.mkv":                                    {"the campaign", 2012, 77953},
		"the.hangover.part.ii.2011.720p.mkv":                                     {"the hangover part ii", 2011, 45243},
		"The Hangover.2009.mp4":                                                  {"The Hangover", 2009, 18785},
		"The.Town.EXTENDED.2010.1080p.BrRip.x264.YIFY.mp4":                       {"The Town", 2010, 0},
		"tropic.thunder.2008.dircut.720p.mkv":                                    {"tropic thunder", 2008, 7446},
		"your.highness.2011.720p.mkv":                                            {"your highness", 2011, 38319},
		"a.scanner.darkly.2006.720p.avi":                                         {"a scanner darkly", 2006, 3509},
		"apollo.13.1995.720p.mkv":                                                {"apollo 13", 1995, 568},
		"borat.1080p.2006.mkv":                                                   {"borat", 2006, 0},
		"catch.me.if.you.can.2002.720p.avi":                                      {"catch me if you can", 2002, 640},
		"christopher.and.his.kind.2011.avi":                                      {"christopher and his kind", 2011, 60170},
		"contagion.2011.720p.mkv":                                                {"contagion", 2011, 39538},
		"cowboys.and.aliens.2011.720p.mkv":                                       {"cowboys and aliens", 2008, 49849},
		"equilibrium.720p.mkv":                                                   {"equilibrium", 1997, 0},
		"john.carter.2012.720p.mkv":                                              {"john carter", 2012, 49529},
		"juno.2007.720p.mkv":                                                     {"juno", 2007, 7326},
		"Man.on.a.Ledge.2012.720p.mkv":                                           {"Man on a Ledge", 2012, 49527},
		"moonrise.kingdom.2012.720p.mkv":                                         {"moonrise kingdom", 2012, 83666},
		"terminator.salvation.2009.1080p.mkv":                                    {"terminator salvation", 2009, 146249},
		"the.dictator.2012.720p.mkv":                                             {"the dictator", 2012, 76493},
		"the.girl.with.the.dragon.tattoo.2011.720p.mkv":                          {"the girl with the dragon tattoo", 2011, 65754},
		"the.ides.of.march.2011.720p.mkv":                                        {"the ides of march", 2011, 10316},
		"the.sitter.unrated.2011.1080p.mp4":                                      {"the sitter", 2011, 0},
		"tower.heist.2011.720p.mkv":                                              {"tower heist", 2011, 59108},
		"zoolander.2001.720p.mkv":                                                {"zoolander", 2001, 9398},
		"the.hunger.games.2012.1080p.mkv":                                        {"the hunger games", 2012, 70160},
		"prometheus.2012.1080p.mkv":                                              {"prometheus", 2012, 70981},
		"ray.2004.1080p.mkv":                                                     {"ray", 2004, 1677},
		"men.in.black.3.2012.720p.mkv":                                           {"men in black 3", 2012, 41154},
		"the.amazing.spider-man.2012.1080p.mkv":                                  {"the amazing spider-man", 2012, 1930},
		"ted.2012.720p.mkv":                                                      {"ted", 2012, 72105},
		"Thor.2011.1080p.mkv":                                                    {"Thor", 2011, 10195},
		"batman.the.dark.knight.rises.2012.mkv":                                  {"batman the dark knight rises", 2012, 49026},
		"swordfish.2001.720p.mkv":                                                {"swordfish", 2001, 9705},
		"Django.Unchained.2012.1080p.bluray.x264-sparks.ShareKiosk.mkv":          {"Django Unchained", 2012, 68718},
		"Gangster.Squad.2013.720p.WEB-DL.AAC2.0.H264-HDClub.mkv":                 {"Gangster Squad", 2013, 82682},
		"In.Bruges.2008.MULTi.720p.BluRay.DTS.x264-SYNERGY.mkv":                  {"In Bruges", 2008, 8321},
		"Super.Troopers.2001.720p.BluRay.DTS.x264-CtrlHD.mkv":                    {"Super Troopers", 2001, 39939},
		"knocked.up.2007.720p.mkv":                                               {"knocked up", 2007, 4964},
		"Shortbus.2006.LIMITED.720p.BluRay.X264-AMIABLE.mkv":                     {"Shortbus", 2006, 1378},
		"Melancholia.2011.720p.BluRay.x264-DON.mkv":                              {"Melancholia", 2011, 62215},
		"argo.2012.720p.bluray.x264-sparks.mkv":                                  {"argo", 2012, 68734},
		"the.american.2010.720p.mkv":                                             {"the american", 2010, 27579},
		"team.america.world.police.2004.720p.mkv":                                {"team america world police", 2004, 3989},
		"taken.2.2012.1080p.mkv":                                                 {"taken 2", 2012, 82675},
		"star.trek.2009.mkv":                                                     {"star trek", 2009, 13475},
		"sunshine.2007.720p.mkv":                                                 {"sunshine", 2007, 1272},
		"super.8.2011.720p.mkv":                                                  {"super 8", 2011, 37686},
		"superbad.2007.720p.mkv":                                                 {"superbad", 2007, 8363},
		"Upstream.Color.2013.1080p.Bluray.X264-BARC0DE.mkv":                      {"Upstream Color", 2013, 145197},
		"Upstream.Color.2013.LIMITED.1080p.BluRay.X264-AMIABLE.mkv":              {"Upstream Color", 2013, 145197},
		"stand.up.guys.2012.1080p.bluray.mkv":                                    {"stand up guys", 2012, 121824},
		"A.Good.Day.To.Die.Hard.2013.720p.WEB-DL.X264-WEBiOS.mkv":                {"A Good Day To Die Hard", 2013, 47964},
		"identity.thief.2013.unrated.720p.web-dl.h264-nogrp.mkv":                 {"identity thief", 2013, 0},        // dupe
		"Seven.Psychopaths.2012.720p.Bluray.x264-BARC0DE.mkv":                    {"Seven Psychopaths", 2012, 86838}, // dupe
		"identity.thief.2013.1080p.bluray.x264-sparks.mkv":                       {"identity thief", 2013, 0},
		"seven.psychopaths.2012.720p.bluray.x264-sparks.mkv":                     {"seven psychopaths", 2012, 86838},
		"Pineapple.Express.2008.720p.BluRay.AC3.x264-HDWinG.mkv":                 {"Pineapple Express", 2008, 10189},
		"Hot Rod 2007 DVDRiP x264 AC3-OFFLiNE.mp4":                               {"Hot Rod", 2007, 10074},
		"the.incredible.burt.wonderstone.2013.720p.bluray.x264-sparks.mkv":       {"the incredible burt wonderstone", 2013, 124459},
		"europa.report.2013.720p.web-dl.h264-publichd.mkv":                       {"europa report", 2013, 174772},
		"gi.joe.retaliation.2013.extended.1080p.bluray.vedett.mkv":               {"gi joe retaliation", 2013, 72559},
		"olympus.has.fallen.2013.720p.bluray.dts.x264-publichd.mkv":              {"olympus has fallen", 2013, 117263},
		"The.Call.2012.720p.BluRay.x264-SPARKS.mkv":                              {"The Call", 2012, 72841},
		"oblivion.2013.1080p.bluray.x264-sparks.mkv":                             {"oblivion", 2013, 75612},
		"Iron.Man.3.2013.720p.HDTV.AC3.x264-TeRRa.mkv":                           {"Iron Man 3", 2013, 68721},
		"Iron Man 2 2010 1080p BDRip AAC x264-tomcat12.mp4":                      {"Iron Man 2", 2010, 10138},
		"Iron Man 2008 1080p BDRip AAC x264-tomcat12.mp4":                        {"Iron Man", 2008, 1726},
		"star.trek.into.darkness.2013.1080p.web-dl.h264-publichd.mkv":            {"star trek into darkness", 2013, 54138},
		"The.Iceman.2012.720p.BluRay.x264-ALLiANCE.mkv":                          {"The Iceman", 2012, 68812},
		"epic.2013.1080p.bluray.dts.x264-publichd.mkv":                           {"epic", 2013, 116711},
		"the.great.gatsby.2013.1080p.bluray.dts.x264-publichd.mkv":               {"the great gatsby", 2013, 0},
		"Jack.Reacher.2012.720p.BluRay.X264-AMIABLE.mkv":                         {"Jack Reacher", 2012, 75780},
		"Furious.6.2013.1080p.BluRay.x264.YIFY.mp4":                              {"Furious 6", 2013, 82992},
		"World.War.Z.2013.Unrated.Cut.720p.BluRay.x264.DTS-WiKi.mkv":             {"World War Z", 2013, 72190},
		"Charlie.Wilsons.War.2007.720p.BluRay.DD5.1.x264-EbP.mkv":                {"Charlie Wilsons War", 2007, 6538},
		"This.is.the.End.2013.1080p.BluRay.x264-SPARKS.mkv":                      {"This is the End", 2013, 109414},
		"After.Earth.2013.1080p.BluRay.DTS.x264-PublicHD.mkv":                    {"After Earth", 2013, 82700},
		"The.Hangover.Part.III.2013.720p.BluRay.DTS.x264-PublicHD.mkv":           {"The Hangover Part III", 2013, 109439},
		"Pacific.Rim.2013.720p.WEB-DL.H264-PublicHD.mkv":                         {"Pacific Rim", 2013, 68726},
		"The.Heat.2013.UNRATED.720p.BluRay.DTS.x264-PublicHD.mkv":                {"The Heat", 2013, 136795},
		"safe.house.2012.720p.mkv":                                               {"safe house", 2012, 59961},
		"red.2010.720p.mkv":                                                      {"red", 2010, 39514},
		"pulp.fiction.1994.720p.mkv":                                             {"pulp fiction", 1994, 680},
	}

	tmdbApi := getTmdbApi(t)

	fis, err := d.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range fis {
		expectedRes, ok := expectedResults[fi.Name()]
		if !ok {
			log.Println("SKIP", fi.Name())
			continue
		}

		title, year := mediautil.ParseMovieFilename(fi.Name())
		if title == "" {
			log.Println("PARSE", "failed")
			continue
		}

		if false { // change this once title/year parsing is totally fixed
			movie := testLookupMovie(t, tmdbApi, title, year)

			if movie == nil {
				log.Println("MATCH", "fail", title, year)
				continue
			}

			if expectedRes.TmdbId != movie.Id {
				log.Println("WRONG TMDBID:", fi.Name(), expectedRes.TmdbId, movie.Id)
				continue
			}

		}

		_ = tmdbApi.DownloadImage

		if expectedRes.Title != title {
			log.Printf(`WRONG TITLE: %s "%s" "%s"`+"\n", fi.Name(), expectedRes.Title, title)
			continue
		}
		if expectedRes.Year != year {
			log.Printf(`WRONG YEAR: %s "%s" "%s"`+"\n", fi.Name(), expectedRes.Year, year)
			continue
		}
	}
}

func testLookupMovie(t *testing.T, tmdbApi *TmdbApi, title string, year int) *Movie {
	results := tmdbApi.LookupMovies(title, year)
	if len(results) > 0 {
		res := results[0]
		return &res
	} else {
		return nil
	}
}
