package owl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// GoModel holds all classes
type GoModel struct {
	Class       map[string]GoClass
	Individual  []GoIndividual
	IRI         string
	Description string
	Name        string
	Content     []byte
	Module      string
	Import      []*GoModel
}

// GoClass holds properties of a class
type GoClass struct {
	IRI          string
	Name         string
	Parent       []string
	DirectParent []string
	Imports      map[string]string
	Property     []GoProperty
	Comment      string
	Model        *GoModel
}

// GoProperty holds condiguration of a property
type GoProperty struct {
	IRI          string      // IRI
	Name         string      // name
	Capital      string      // name with capital first letter
	Typ          [2]string   // type in struct (child of BaseTyp, 0: typ; 1: IRI)
	BaseTyp      [2]string   // type in interface
	AllowedTyp   [][2]string // allowed types (children of Typ)
	XSDTyp       string      // XSD type if any
	Individual   []string
	Multi        bool
	Multiplicity string
	Comment      string
	Inverse      string
	Symmetric    bool
}

// GoIndividual individuals
type GoIndividual struct {
	IRI        string
	Name       string
	Typ        string
	ImportName string
}

// MapModel extracts the model from an ontology
func MapModel(ont *Ontology, moduleName string) (mod []GoModel, err error) {
	mod = make([]GoModel, len(ont.Imports), len(ont.Imports))
	index := 0
	for i := range ont.Imports {
		mod[index].IRI = i
		mod[index].Description = ont.Description[i]
		mod[index].Module = moduleName
		temp := strings.Split(i, "/")
		mod[index].Name = temp[len(temp)-1]
		index++
	}

	for i := range mod {
		for j := range ont.Imports[mod[i].IRI] {
			for k := range mod {
				if mod[k].IRI == ont.Imports[mod[i].IRI][j] {
					mod[i].Import = append(mod[i].Import, &mod[k])
				}
			}
		}
		err = mod[i].createGoClasses(ont)
		if err != nil {
			return
		}

		for j := range ont.Individual {
			if strings.HasPrefix(ont.Individual[j].Type.Name, mod[i].IRI+"#") {
				temp := mod[i].extractIndividual(ont.Individual[j], ont)
				if temp.IRI != "" {
					mod[i].Individual = append(mod[i].Individual, temp)
				}
			}
		}
	}

	return
}

// createGoClasses creates all necessary GoClasses and fills their information if possible
func (mod *GoModel) createGoClasses(ont *Ontology) (err error) {
	mod.Class = make(map[string]GoClass)
	for i := range ont.Class {
		var temp GoClass
		temp, err = mod.extractClass(ont.Class[i], ont)
		if err != nil {
			return
		}
		if temp.Name != "" {
			mod.Class[temp.Name] = temp
		}
	}
	return
}

// extractClass extracts a single class
func (mod *GoModel) extractClass(class *Class, ont *Ontology) (goClass GoClass, err error) {
	goClass.Imports = make(map[string]string)
	// get class name and IRI
	if !strings.HasPrefix(class.Name, mod.IRI+"#") {
		//err = errors.New("wrong iri: " + class.Name)
		return
	}
	goClass.Name = strings.TrimPrefix(class.Name, mod.IRI+"#")
	goClass.IRI = class.Name

	// get comment
	goClass.Comment = class.Comment

	// get parents
	parents := class.GetAllParents()
	for i := range parents {
		parentName, importName := trimName(parents[i].Name, ont)
		if parentName == "" {
			err = errors.New("Class " + class.Name + ": wrong parent: " + parents[i].Name)
			return
		}
		if parentName != goClass.Name {
			if !strings.HasSuffix(mod.IRI, importName) {
				parentName = "im" + importName + "." + parentName
			}
			goClass.Parent = append(goClass.Parent, parentName)
		}
	}
	for i := range class.Parent {
		parentName, importName := trimName(class.Parent[i].Name, ont)
		if parentName == "" {
			err = errors.New("Class " + class.Name + ": wrong parent: " + class.Parent[i].Name)
			return
		}
		if parentName != goClass.Name {
			if !strings.HasSuffix(mod.IRI, importName) {
				parentName = "im" + importName + "." + parentName
			}
			goClass.DirectParent = append(goClass.DirectParent, parentName)
		}
	}

	// get properties
	restInv := class.GetRestrictionsInverse()
	rest := class.GetRestrictions()
	for i := range restInv {
		var property GoProperty
		property.Name, property.Capital, property.IRI = getRestrictionNameAndIRI(restInv[i], ont)
		if property.Name == "" {
			err = errors.New("Class " + class.Name + " unknown property " + restInv[i].Property.Name)
			return
		}
		if _, ok := ont.Property[restInv[i].Property.Name]; ok {
			property.Comment = ont.Property[restInv[i].Property.Name].Comment
		}
		var exist bool
		var im string
		property.BaseTyp, exist, im = getRestrictionType(restInv[i], ont)
		if !exist {
			err = errors.New("Class " + class.Name + " Restriction " + restInv[i].Property.Name + " unknown base type " + property.BaseTyp[0])
			return
		}
		if im != "" && !strings.HasSuffix(mod.IRI, im) {
			if property.Multi {
				goClass.Imports[mod.Module+"/pkg/"+im] = "im" + im + " "
			}
			property.BaseTyp[0] = "im" + im + "." + property.BaseTyp[0]
		}
		property.Multi, property.Multiplicity = getRestrictionMultiplicity(restInv[i])
		if restInv[i].Property.Inverse != nil {
			temp, _ := trimName(restInv[i].Property.Inverse.Name, ont)
			property.Inverse = strings.Title(temp)
		}

		for j := range rest {
			if rest[j].Property.Name == restInv[i].Property.Name {
				property.Typ, exist, im = getRestrictionType(rest[j], ont)
				if !exist || property.Typ[0] == "string" {
					// fmt.Println("Warning: Class " + class.Name + " Restriction " + rest[j].Property.Name + " unknown type " + property.Typ)
					property.Typ = property.BaseTyp
				}
				if im != "" && !strings.HasSuffix(mod.IRI, im) {
					if property.Multi || property.Inverse != "" {
						goClass.Imports[mod.Module+"/pkg/"+im] = "im" + im + " "
					}
					property.Typ[0] = "im" + im + "." + property.Typ[0]
				}
				//if property.Typ == "string" || property.Typ == "int" || property.Typ == "float64" ||
				if property.Typ[0] == "time.Time" || property.Typ[0] == "time.Duration" {
					if len(rest[j].Value) > 0 {
						property.XSDTyp = rest[j].Value[0]
					} else if len(restInv[i].Value) > 0 {
						property.XSDTyp = restInv[i].Value[0]
					} else {
						err = errors.New("Class " + class.Name + " Restriction " + restInv[i].Property.Name + " no xsd type")
						return
					}
				}
				allowedType, importNames := getRestrictionAllowedTypes(rest[j], ont)
				if len(allowedType) == 0 {
					property.AllowedTyp = append(property.AllowedTyp, property.Typ)
				} else {
					for k := range allowedType {
						if !strings.HasSuffix(mod.IRI, importNames[k]) {
							allowedType[k][0] = "im" + importNames[k] + "." + allowedType[k][0]
							property.AllowedTyp = append(property.AllowedTyp, allowedType[k])
						} else {
							property.AllowedTyp = append(property.AllowedTyp, allowedType[k])
						}
					}
				}
				property.Individual, err = getIndividuals(rest[j], ont)
			}
		}
		if b, _ := GetBaseClass([]string{property.BaseTyp[1], property.Typ[1]}, ont.Class); property.BaseTyp[0] != property.Typ[0] && b == nil {
			// WARNING: This removes properties that are present in specification (relevant for saref4ener:PowerProfile:consists of; saref4ener:AlternativesGroup does not inherit from saref:Profile)
			continue
		}
		goClass.Property = append(goClass.Property, property)
	}

	return
}

func getRestrictionNameAndIRI(rest *Restriction, ont *Ontology) (name string, capital string, iri string) {
	name, _ = trimName(rest.Property.Name, ont)
	iri = rest.Property.Name
	capital = strings.Title(name)
	return
}

func getRestrictionMultiplicity(rest *Restriction) (multi bool, multiplicity string) {
	if rest.ValueConstraint == "http://www.w3.org/2002/07/owl#allValuesFrom" ||
		rest.ValueConstraint == "http://www.w3.org/2002/07/owl#someValuesFrom" ||
		rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#minQualifiedCardinality" ||
		rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#minCardinality" {
		multi = true
		multiplicity = "[]"
	} else if rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#maxCardinality" ||
		rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#qualifiedCardinality" ||
		rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#maxQualifiedCardinality" ||
		rest.CardinalityConstraint == "http://www.w3.org/2002/07/owl#cardinality" {
		if rest.Multiplicity > 1 {
			multi = true
			multiplicity = "[" + strconv.Itoa(rest.Multiplicity) + "]"
		} else {
			multi = false
			multiplicity = ""
		}
	}
	return
}

func getRestrictionAllowedTypes(rest *Restriction, ont *Ontology) (values [][2]string, importNames []string) {
	if rest.ValueConstraint == "http://www.w3.org/2002/07/owl#hasValue" {
		base, err := GetBaseType(rest.Value, ont.Individual, ont.Class)
		if err != nil {
			return
		}
		if base != nil {
			allowedType, importName := trimName(base.Name, ont)
			values = append(values, [2]string{allowedType, base.Name})
			importNames = append(importNames, importName)
		} else {
			fmt.Println("unknown basetype for " + rest.Property.Name)
		}
	} else {
		for i := range rest.Value {
			if _, ok := ont.Class[rest.Value[i]]; ok {
				allowedType, importName := trimName(rest.Value[i], ont)
				values = append(values, [2]string{allowedType, rest.Value[i]})
				importNames = append(importNames, importName)
			} else if strings.HasPrefix(rest.Value[i], "http://www.w3.org/2001/XMLSchema") {
				allowedType, err := mapLiteralType(rest.Value[i])
				if err == nil {
					values = append(values, [2]string{allowedType, ""})
					importNames = append(importNames, "")
				}
			}
		}
	}
	return
}

// getRestrictionType returns restriction type
func getRestrictionType(rest *Restriction, ont *Ontology) (ret [2]string,
	typeExist bool, importName string) {
	typeExist = false
	if rest.ValueConstraint == "http://www.w3.org/2002/07/owl#hasValue" {
		base, err := GetBaseType(rest.Value, ont.Individual, ont.Class)
		if err != nil {
			err = errors.New("Restriction " + rest.Property.Name + " " + fmt.Sprint(err))
			return
		}
		if base != nil {
			ret[0], _ = trimName(base.Name, ont)
		} else {
			ret[0] = "owl.Thing"
		}
		typeExist = true
	} else {
		isClass := false
		isLiteral := false
		for i := range rest.Value {
			if temp, _ := trimName(rest.Value[i], ont); temp != "" {
				isClass = true
			} else if strings.HasPrefix(rest.Value[i], "http://www.w3.org/2001/XMLSchema") {
				isLiteral = true
			}
		}
		if isClass && !isLiteral {
			base, err := GetBaseClass(rest.Value, ont.Class)
			if err != nil {
				err = errors.New("Restriction " + rest.Property.Name + " " + fmt.Sprint(err))
				return
			}
			if base != nil {
				ret[0], importName = trimName(base.Name, ont)
				ret[1] = base.Name
			} else {
				ret[0] = "owl.Thing"
			}
			typeExist = true
		} else if isLiteral && !isClass {
			var err error
			ret[0], err = mapLiteralType(rest.Value[0])
			if err == nil {
				typeExist = true
			}
		} else {
			ret[0] = "string"
			typeExist = true
		}
	}

	return
}

func getIndividuals(rest *Restriction, ont *Ontology) (inds []string, err error) {
	if rest.ValueConstraint == "http://www.w3.org/2002/07/owl#hasValue" {
		for i := range rest.Value {
			inds = append(inds, rest.Value[i])
		}
	}
	return
}

// extractIndividual
func (mod *GoModel) extractIndividual(individual *Individual, ont *Ontology) (goIndividual GoIndividual) {
	if temp, _ := trimName(individual.Name, ont); temp != "" {
		goIndividual.Name = temp
	} else if strings.HasPrefix(individual.Name, "http://www.wurvoc.org/vocabularies/om-1.8") {
		goIndividual.Name = strings.TrimPrefix(individual.Name, "http://www.wurvoc.org/vocabularies/om-1.8/")
	}
	if goIndividual.Name == "" {
		return
	}
	temp := strings.Split(goIndividual.Name, "_")
	for i := range temp {
		temp[i] = strings.Title(temp[i])
	}
	goIndividual.Name = strings.Join(temp, "")
	goIndividual.IRI = individual.Name
	goIndividual.Typ, goIndividual.ImportName = trimName(individual.Type.Name, ont)
	return
}

// mapLiteralType maps the literal type to a go datatype
func mapLiteralType(literal string) (goType string, err error) {
	switch literal {
	case "http://www.w3.org/2001/XMLSchema#dateTime":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#date":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#duration":
		goType = "time.Duration"
	case "http://www.w3.org/2001/XMLSchema#dateTimeStamp":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#gYear":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#gDay":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#gYearMonth":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#gMonth":
		goType = "time.Time"
	case "http://www.w3.org/2001/XMLSchema#string":
		goType = "string"
	case "http://www.w3.org/2001/XMLSchema#float":
		goType = "float64"
	case "http://www.w3.org/2001/XMLSchema#decimal":
		goType = "float64"
	case "http://www.w3.org/2001/XMLSchema#integer":
		goType = "int"
	case "http://www.w3.org/2001/XMLSchema#nonNegativeInteger":
		goType = "int"
	case "http://www.w3.org/2001/XMLSchema#unsignedInt":
		goType = "int"
	case "http://www.w3.org/2001/XMLSchema#boolean":
		goType = "bool"
	default:
		err = errors.New("Unknown literal")
	}
	return
}

func trimName(name string, ont *Ontology) (out string, imp string) {
	for i := range ont.Imports {
		if strings.HasPrefix(name, i+"#") {
			out = strings.TrimPrefix(name, i+"#")
			temp := strings.Split(i, "/")
			imp = temp[len(temp)-1]
			return
		}
	}
	out = ""
	imp = ""
	return
}