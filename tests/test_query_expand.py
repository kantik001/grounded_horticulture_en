from rag.query_expand import expand_query


def test_expand_marssoniosis():
    # Glossary keys remain multilingual; Russian query still expands via agro_glossary.json.
    out = expand_query("What is known about marssonina on apple?").lower()
    assert "marssonina" in out


def test_expand_no_change_without_match():
    q = "How to irrigate pear trees?"
    assert expand_query(q) == q


def test_expand_rootstock_synonyms():
    out = expand_query("Liberty on knip").lower()
    assert "liberty" in out
    assert "knip" in out
