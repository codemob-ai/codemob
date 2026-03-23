package mob

import "math/rand"

var adjectives = []string{
	"angry", "brave", "calm", "daring", "eager",
	"fancy", "gentle", "happy", "icy", "jolly",
	"keen", "lazy", "mighty", "noble", "odd",
	"proud", "quiet", "rapid", "shy", "tiny",
	"uber", "vivid", "warm", "xenial", "young",
	"zany", "bold", "crisp", "dark", "epic",
	"fierce", "grand", "harsh", "iron", "juicy",
	"kind", "loud", "mellow", "neat", "orange",
	"plain", "quick", "rough", "smooth", "tough",
	"ultra", "vast", "wild", "exact", "zippy",
}

var fruits = []string{
	"apple", "banana", "cherry", "date", "elderberry",
	"fig", "grape", "honeydew", "jackfruit", "kiwi",
	"lemon", "mango", "nectarine", "olive", "papaya",
	"quince", "raspberry", "strawberry", "tomato", "ugli",
	"vanilla", "watermelon", "ximenia", "yuzu", "zucchini",
	"apricot", "blueberry", "coconut", "dragonfruit", "eggplant",
	"fennel", "guava", "habanero", "jalapeno", "kumquat",
	"lime", "mulberry", "nutmeg", "onion", "pear",
	"radish", "sage", "turnip", "arugula", "basil",
	"celery", "dill", "endive", "garlic", "horseradish",
}

// GenerateName creates a random adjective-fruit name.
func GenerateName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	fruit := fruits[rand.Intn(len(fruits))]
	return adj + "-" + fruit
}
