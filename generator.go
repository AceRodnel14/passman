package main

import (
	"strings"
)

const (
	minGenLength = 8
	maxGenLength = 84
)

// Look-alike substitutions. When both are enabled the symbol wins for
// letters that have both (a, i, l, s).
var digitSubs = map[rune]rune{
	'o': '0', 'i': '1', 'l': '1', 'z': '2', 'e': '3', 'a': '4',
	's': '5', 'g': '6', 't': '7', 'b': '8', 'q': '9',
}

var symbolSubs = map[rune]rune{
	's': '$', 'l': '|', 'a': '@', 'i': '!', 'c': '(', 'h': '#',
}

var wordsByLen = func() map[int][]string {
	m := map[int][]string{}
	for _, w := range strings.Fields(wordList) {
		m[len(w)] = append(m[len(w)], w)
	}
	return m
}()

const (
	minWordLen = 3
	maxWordLen = 8
)

// generatePassphrase returns capitalised words joined together, exactly
// length characters long, then applies the optional look-alike swaps to
// every matching letter.
func generatePassphrase(length int, numbers, symbols bool) string {
	var b strings.Builder
	for remaining := length; remaining > 0; {
		// Choose among words that fit and leave either nothing or room for
		// at least one more word.
		var candidates []string
		for l := minWordLen; l <= maxWordLen && l <= remaining; l++ {
			if rest := remaining - l; rest == 0 || rest >= minWordLen {
				candidates = append(candidates, wordsByLen[l]...)
			}
		}
		w := candidates[randomIntn(len(candidates))]
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		remaining -= len(w)
	}

	out := []rune(b.String())
	for i, r := range out {
		lower := []rune(strings.ToLower(string(r)))[0]
		if symbols {
			if s, ok := symbolSubs[lower]; ok {
				out[i] = s
				continue
			}
		}
		if numbers {
			if d, ok := digitSubs[lower]; ok {
				out[i] = d
			}
		}
	}
	return string(out)
}

const wordList = `
ace act age aim air ant ape arc arm art ash axe bay bee box bud bus cab cat cog cow cub cup day den dew dog dot dry dye ear elk elm emu eve eye fan fig fin fir fix fly fog fox fun gem gnu gum hat hay hen hip hog hop hub hue ice ink ivy jam jar jaw jet jog joy key kid kin kit lab lap law leg lid lip log map mat mix mop mud mug net nut oak oar oat oil orb owl pad pan paw pea pen pie pig pin pod pot ram ray rib rim rod row rug rye sap saw sea sky sod soy spa sun tab tan tea tin toe ton toy tub urn van vat vet web wig yak yam zen zip

acre aqua arch atom aunt axis bark barn bead beam bean bear bell belt bird blue boat bolt bone book boot bowl brew bulb cake calm camp cape card cart cave cell chef chin clay clip coal coat code coin cola cone cook cord corn crab crew crop cube curl dawn deer desk dime disk dock dome door dove drum duck dune dust echo edge envy epic fawn fern film fire fish flag foam fork frog fuel gate gear gift glow goat gold golf gulf hail hall harp hawk heat herb hill hive hook horn iris iron isle jade jazz kale kelp kite kiwi knot lake lamb lamp lane lava leaf lime lion loaf loft loop lute mail malt mane mars mast maze meal mesa mile milk mint mist moat mole moon moss moth nest note nova oath opal oven palm park path peak pear pine pipe plum pond pony port quay quiz raft rain reef rice ring road rock roof rose ruby sage sail salt sand seal seed ship silk sled snow soap sock sofa soil song soup spur star stem swan tide tile toad tram tree tuba tuna twig vase veil vest vine volt wand wasp wave well whey wind wing wolf wood wool yard yarn yeti yoga zinc zone

acorn adobe alarm amber angel anvil apple apron arrow aspen atlas attic bacon badge bagel baker basil beach berry bison blade blaze bloom board bongo brick brook brush cabin cable camel candy canoe cargo cedar chalk charm chess chili cider cloud clove coast cobra comet coral crane crate creek crown daisy delta denim diary dingo dough dream eagle earth easel ember fable fairy feast fence ferry field finch flame flask flint flute focus forge frost fudge gecko ghost giant glade glass globe grain grape grass gravy guava guide happy hazel heron honey horse igloo inlet ivory jelly jewel joker judge juice kayak koala ladle lemon lilac linen llama lodge lotus lunar mango maple marsh medal melon mocha model moose motor mount mouse nacho noble north ocean olive onion orbit otter panda paper pasta peach pearl pecan pedal penny piano pilot pixel pizza plaza poppy prism quail quart quilt radar radio raven river robin rodeo royal salad scarf scout shelf shell shore sigma skate slate sloth smile snail solar spice spoon squid stamp steam stone storm sugar swift table tango tiger toast topaz torch tower trail tulip umber union valve vapor vault venus vigor viola vivid waltz water whale wheat world yacht yodel zebra

almond anchor answer arctic autumn badger bakery bamboo banana barley basket beacon beaver bishop blazer bottle branch breeze bridge bronze bubble bucket butter button cactus camera candle canyon carbon carpet castle cherry chorus cinema circle citrus clever cobalt coffee comet compass cookie copper cotton cradle crayon cricket crystal custom dahlia dancer desert dinner dragon effort engine falcon feather fennel fiddle forest fossil frozen galaxy garden garlic ginger glider goblin granite gravel guitar hammer harbor helmet hermit hiking hollow honest island jaguar jasper jersey jigsaw jungle kernel kettle kitten ladder lagoon lantern laptop lizard locket magnet mammal marble meadow mellow mentor meteor mirror monkey muffin museum nectar needle nickel noodle nutmeg oyster paddle parrot pebble pencil pepper pillow planet pocket potato puzzle rabbit radish rafter ribbon rocket saddle salmon sandal scroll season shadow silver singer sketch spirit spring sprout square stable statue summer sunset tablet tavern thread timber tomato tunnel turtle velvet violet walnut wander window winter wizard yellow zenith

absolute acrobat admiral almanac anthem aquarium balloon banquet battery blanket blossom bouquet buffalo cabbage caravan cartoon cashew chapter chimney chowder circuit compass concert cottage cupcake cypress diamond dolphin eclipse elegant emerald feather festival fiction firefly flannel flowers fortune freedom gallery general giraffe glacier harmony harvest horizon iceberg journey jukebox kingdom kitchen lantern leopard library lobster machine mailbox mammoth mandolin marimba mermaid migrant minute monarch morning mustard network nucleus oatmeal octopus orchard origami panther pelican penguin phoenix picture pilgrim pioneer plateau popcorn pottery program pumpkin pyramid quarter rainbow rhythm sapphire sardine sherbet shuttle skyline society spinach station sunrise teacher thunder tornado trumpet unicorn uniform vintage voyager weather whistle

aardvark airplane alphabet antelope backpack baseball beekeeper blizzard bluebird bookcase borrowed buttercup calendar cardinal carousel champion chipmunk chocolate cinnamon coconut composer cucumber daydream dinosaur doorbell dumpling elephant envelope evergreen fireside flamingo football fountain gardener goldfish graphite hedgehog highland homemade hotelier hummingbird icicle kangaroo keyboard lavender lemonade lighthouse magazine mandarin marathon meatball midnight molasses mosquito mountain mushroom notebook observer olympics overture painting pancake parakeet peaceful peppermint pinecone platinum porridge portrait pretzel quantum question railroad raindrop reindeer sandwich scenario seashell shamrock skeleton snowball snowfall sparrow spectrum squirrel stardust strategy sunlight sweater teaspoon thinking tortoise treasure triangle umbrella vacation velocity volcano walnuts waterfall windmill woodland yearbook zucchini
`
